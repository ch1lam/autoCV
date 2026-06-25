#!/usr/bin/env python3
from __future__ import annotations

import argparse
import pathlib
import sys
from dataclasses import dataclass
from urllib.parse import urlparse

import pypdfium2 as pdfium
import weasyprint
from weasyprint import HTML
from weasyprint.urls import FatalURLFetchingError, URLFetcher


APP_VERSION = "0.1.0"


@dataclass(frozen=True)
class RenderOptions:
    html: pathlib.Path
    pdf: pathlib.Path
    preview_dir: pathlib.Path
    asset_root: pathlib.Path | None
    scale: float


class RestrictedFetcher(URLFetcher):
    def __init__(self, asset_root: pathlib.Path | None) -> None:
        super().__init__(timeout=5)
        self.asset_root = asset_root.resolve() if asset_root else None

    def fetch(self, url: str, headers=None):
        parsed = urlparse(url)
        if parsed.scheme in ("http", "https"):
            raise FatalURLFetchingError("remote resources are not allowed")
        if parsed.scheme == "file" and self.asset_root is not None:
            path = pathlib.Path(parsed.path).resolve()
            if path != self.asset_root and self.asset_root not in path.parents:
                raise FatalURLFetchingError("file resource is outside asset root")
        return super().fetch(url, headers=headers)


def render(options: RenderOptions) -> None:
    html_path = options.html.resolve()
    pdf_path = options.pdf.resolve()
    preview_dir = options.preview_dir.resolve()
    preview_dir.mkdir(parents=True, exist_ok=True)
    pdf_path.parent.mkdir(parents=True, exist_ok=True)

    base_url = options.asset_root.resolve() if options.asset_root else html_path.parent
    document = HTML(
        filename=str(html_path),
        base_url=str(base_url),
        url_fetcher=RestrictedFetcher(options.asset_root),
    )
    document.write_pdf(str(pdf_path))
    if not pdf_path.exists() or pdf_path.stat().st_size == 0:
        raise RuntimeError("WeasyPrint did not create a PDF")

    pdf = pdfium.PdfDocument(str(pdf_path))
    try:
        if len(pdf) == 0:
            raise RuntimeError("rendered PDF has no pages")
        for index in range(len(pdf)):
            page = pdf[index]
            bitmap = page.render(scale=options.scale)
            image = bitmap.to_pil()
            image.save(preview_dir / f"page-{index + 1}.png")
    finally:
        pdf.close()


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="autocv-pdf-renderer")
    parser.add_argument("--version", action="store_true")
    subparsers = parser.add_subparsers(dest="command")

    render_parser = subparsers.add_parser("render")
    render_parser.add_argument("--html", required=True)
    render_parser.add_argument("--pdf", required=True)
    render_parser.add_argument("--preview-dir", required=True)
    render_parser.add_argument("--asset-root")
    render_parser.add_argument("--scale", type=float, default=2.0)
    return parser.parse_args(argv)


def version() -> str:
    return (
        f"autocv-pdf-renderer {APP_VERSION}; "
        f"weasyprint {weasyprint.__version__}; "
        f"pypdfium2 {pdfium.__version__}"
    )


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    if args.version:
        print(version())
        return 0
    if args.command != "render":
        print("missing command: render", file=sys.stderr)
        return 2

    try:
        render(
            RenderOptions(
                html=pathlib.Path(args.html),
                pdf=pathlib.Path(args.pdf),
                preview_dir=pathlib.Path(args.preview_dir),
                asset_root=pathlib.Path(args.asset_root) if args.asset_root else None,
                scale=args.scale,
            )
        )
    except Exception as exc:  # pragma: no cover - process boundary reports stderr.
        print(f"render failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
