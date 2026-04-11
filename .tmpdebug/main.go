package main

import (
  "fmt"
  "strings"
  req "pipnest/internal/requirements"
  tea "github.com/charmbracelet/bubbletea"
)

func main() {
  m := req.NewViewModel()
  m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
  out := m.View()
  lines := strings.Split(out, "\n")
  for i := 0; i < 3 && i < len(lines); i++ {
    fmt.Printf("L%d len=%d %q\n", i, len([]rune(lines[i])), lines[i])
  }
}
