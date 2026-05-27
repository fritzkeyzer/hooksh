# run xml output commands and pipe them into index.xml
hooksh entrypoints --format call-tree --limit 20 --depth 2 --functions --exported-only > demo/index.xml
hooksh packages --kind go-package-doc --limit 10 >> demo/index.xml
hooksh docs --kind md --limit 10 >> demo/index.xml

# generate dot diagram
hooksh go-lyze --format dot --top "cmd/hooksh" --output demo/graph.dot
# convert dot to png using `dot` cli
cat demo/graph.dot | dot -Tpng -o demo/graph.dot.png

# generate mermaid
hooksh go-lyze --format mermaid --top "cmd/hooksh" --output demo/graph.mermaid

# generate mermaid + html
hooksh go-lyze --format mermaid --top "cmd/hooksh" --output demo/graph.mermaid --html demo/graph.html

# generate html-render only
hooksh go-lyze --format mermaid --top "cmd/hooksh" --html-render demo/graph.html.png

# generate mermaid + html viewer + png render (realistic usage)
hooksh go-lyze --format mermaid --top "cmd/hooksh" \
  --output demo/graph.mermaid \
  --html demo/graph.dark.html \
  --html-dark \
  --html-node-color "xmlutil,#eb34de" \
  --html-hidden-nodes "limitutil,fsutil" \
  --html-layout "LR" \
  --html-title "Go package structure" \
  --html-subtitle "With the cli main package as the root node (LR layout, hidden and colored nodes for demo purposes)" \
  --html-render demo/graph.dark.html.png \
  --skip-unchanged \
  --html-render-res "900,900"
