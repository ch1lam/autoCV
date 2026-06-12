#let data = json("resume.json")

#set page(
  paper: "a4",
  margin: (x: 18mm, y: 16mm),
  fill: rgb("#ffffff"),
)
#set text(
  font: data.fonts,
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
    size: 10.5pt,
    weight: 700,
    tracking: 0.05em,
    fill: rgb("#23445f"),
  )[#title]
  #v(2pt)
  #line(length: 100%, stroke: 0.65pt + rgb("#8fa0ad"))
]

#align(left)[
  #text(
    size: 21pt,
    weight: 700,
    fill: rgb("#162f45"),
  )[#data.target_role]
]
#v(4pt)
#line(length: 100%, stroke: 1.1pt + rgb("#c36b3c"))

#for section in data.sections {
  section-heading(section.heading)
  for item in section.items {
    if item.kind == "bullet" {
      list.item(text(item.text))
    } else {
      par(text(item.text))
    }
  }
}
