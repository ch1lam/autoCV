#let data = json("resume.json")

#set page(
  paper: "a4",
  margin: (x: 18mm, y: 16mm),
  fill: rgb("#ffffff"),
)
#set text(
  font: data.body_fonts,
  size: 9.8pt,
  fill: rgb("#202b36"),
  lang: if data.language == "zh" { "zh" } else { "en" },
)
#set par(
  justify: true,
  leading: 0.62em,
  spacing: 4pt,
)
#set list(
  indent: 1em,
  body-indent: 0.55em,
  spacing: 3pt,
)

#let section-heading(title) = block(
  above: 10pt,
  below: 5pt,
  breakable: false,
)[
  #text(
    font: data.heading_fonts,
    size: 10.5pt,
    weight: 700,
    tracking: 0.05em,
    fill: rgb("#23445f"),
  )[#title]
  #v(2pt)
  #line(length: 100%, stroke: 0.65pt + rgb("#8fa0ad"))
]

#let item-content(item) = {
  for run in item.runs {
    if run.kind == "link" {
      link(run.url)[
        #text(fill: rgb("#1f5f8b"))[#run.text]
      ]
    } else {
      text(run.text)
    }
  }
}

#let render-item(item) = if item.kind == "bullet" {
  list.item(item-content(item))
} else {
  par(item-content(item))
}

#align(left)[
  #text(
    font: data.heading_fonts,
    size: 21pt,
    weight: 700,
    fill: rgb("#162f45"),
  )[#data.target_role]
]
#v(4pt)
#line(length: 100%, stroke: 1.1pt + rgb("#c36b3c"))

#for section in data.sections {
  for (index, item) in section.items.enumerate() {
    if index == 0 {
      block(breakable: false)[
        #section-heading(section.heading)
        #render-item(item)
      ]
    } else {
      render-item(item)
    }
  }
}
