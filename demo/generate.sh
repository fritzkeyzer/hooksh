# run xml output commands and pipe them into index.xml
go run cmd/hooksh/main.go entrypoints --format call-tree --limit 20 --depth 2 --functions --exported-only > demo/index.xml
go run cmd/hooksh/main.go packages --kind go-package-doc --limit 10 >> demo/index.xml
go run cmd/hooksh/main.go docs --kind md --limit 10 >> demo/index.xml

# generate dot diagram
go run cmd/hooksh/main.go go-lyze --format dot --top "cmd/hooksh" --output demo/graph.dot
# convert dot to png using `dot` cli
cat demo/graph.dot | dot -Tpng -o demo/graph.dot.png

# generate mermaid
go run cmd/hooksh/main.go go-lyze --format mermaid --top "cmd/hooksh" --output demo/graph.mermaid

# generate mermaid + html
go run cmd/hooksh/main.go go-lyze --format mermaid --top "cmd/hooksh" --output demo/graph.mermaid --html demo/graph.html

# generate html-render only
go run cmd/hooksh/main.go go-lyze --format mermaid --top "cmd/hooksh" --html-render demo/graph.html.png

# generate mermaid + html (dark mode) + html render
go run cmd/hooksh/main.go go-lyze --format mermaid --top "cmd/hooksh" \
  --output demo/graph.mermaid \
  --html demo/graph.dark.html \
  --html-dark \
  --html-node-color "xmlutil,#eb34de" \
  --html-hidden-nodes "limitutil,fsutil" \
  --html-layout "LR" \
  --html-title "Go package structure" \
  --html-subtitle "With the cli main package as the root node (LR layout, hidden and colored nodes for demo purposes)" \
  --html-render demo/graph.dark.html.png \
  --html-render-res "900,900"
