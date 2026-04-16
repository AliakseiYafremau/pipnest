package ui

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
)

// AppModel is a minimal, backend-agnostic Bubble Tea model for the TUI layer.
//
// It stores:
// - available backends (string slice) and the currently selected backend index
// - available virtual environments (string slice) and the currently selected venv index
// - a status/message area and an optional error value
// The model is intentionally simple so it can be wired to a service layer via
// NewAppModel(service interface{}).
type AppModel struct {
    // service is kept as an empty interface so callers can pass their own
    // service object. The model does not call any methods on it directly — this
    // keeps the model backend-agnostic and easy to mock in tests.
    service interface{}

    // UI state
    backends        []string
    selectedBackend int // -1 means none selected

    venvs        []string
    selectedVenv int // -1 means none selected

    // status is a short single-line status or message (info, hint, etc.)
    status string

    // lastErr holds the last operation error, not shown automatically but
    // accessible through methods.
    lastErr error

    // requestedQuit set when the model should exit
    requestedQuit bool
}

// NewAppModel constructs a new AppModel wired with the provided service.
// service is stored as-is and can later be type-asserted by callers that
// orchestrate behavior between the TUI and domain/service layers.
func NewAppModel(service interface{}) *AppModel {
    return &AppModel{
        service:         service,
        backends:        nil,
        selectedBackend: -1,
        venvs:           nil,
        selectedVenv:    -1,
        status:          "",
        lastErr:         nil,
    }
}

// ---- Bubble Tea methods ----

// Init implements tea.Model. No initial commands by default.
func (m *AppModel) Init() tea.Cmd {
    return nil
}

// Update implements tea.Model. It handles a few common key messages:
// - q, ctrl+c: request quit.
// Other messages are not handled here; higher-level screens can extend
// behavior by embedding or calling into this model.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            m.requestedQuit = true
            return m, tea.Quit
        }
    }
    return m, nil
}

// View renders a compact representation of the model.
// It's intentionally minimal and styling-free so it can be wrapped by screens.
func (m *AppModel) View() string {
    var sb strings.Builder

    // header
    sb.WriteString("Backends:\n")
    if len(m.backends) == 0 {
        sb.WriteString("  (none detected)\n")
    } else {
        for i, b := range m.backends {
            prefix := "  "
            if i == m.selectedBackend {
                prefix = "→ "
            }
            sb.WriteString(fmt.Sprintf("%s%s\n", prefix, b))
        }
    }

    sb.WriteString("\nEnvironments:\n")
    if len(m.venvs) == 0 {
        sb.WriteString("  (no environments)\n")
    } else {
        for i, v := range m.venvs {
            prefix := "  "
            if i == m.selectedVenv {
                prefix = "→ "
            }
            sb.WriteString(fmt.Sprintf("%s%s\n", prefix, v))
        }
    }

    sb.WriteString("\nStatus:\n")
    if m.status == "" {
        sb.WriteString("  (idle)\n")
    } else {
        sb.WriteString("  " + m.status + "\n")
    }

    if m.lastErr != nil {
        sb.WriteString(fmt.Sprintf("\nLast error: %v\n", m.lastErr))
    }

    return sb.String()
}

// ---- State mutation helpers (exported) ----

// SetBackends replaces the list of available backends and resets the selection
// if the previous selection is out of range.
func (m *AppModel) SetBackends(list []string) {
    m.backends = append([]string(nil), list...)
    if len(m.backends) == 0 {
        m.selectedBackend = -1
        return
    }
    if m.selectedBackend < 0 || m.selectedBackend >= len(m.backends) {
        m.selectedBackend = 0
    }
}

// SetSelectedBackend selects a backend by index. Returns an error for invalid index.
func (m *AppModel) SetSelectedBackend(index int) error {
    if index < 0 || index >= len(m.backends) {
        return fmt.Errorf("backend index out of range")
    }
    m.selectedBackend = index
    return nil
}

// SelectedBackend returns the currently selected backend and a bool indicating
// whether a selection exists.
func (m *AppModel) SelectedBackend() (string, bool) {
    if m.selectedBackend < 0 || m.selectedBackend >= len(m.backends) {
        return "", false
    }
    return m.backends[m.selectedBackend], true
}

// SetVenvs replaces the list of virtual environments and resets the selection
// if the previous selection is out of range.
func (m *AppModel) SetVenvs(list []string) {
    m.venvs = append([]string(nil), list...)
    if len(m.venvs) == 0 {
        m.selectedVenv = -1
        return
    }
    if m.selectedVenv < 0 || m.selectedVenv >= len(m.venvs) {
        m.selectedVenv = 0
    }
}

// SetSelectedVenv selects a venv by index. Returns an error for invalid index.
func (m *AppModel) SetSelectedVenv(index int) error {
    if index < 0 || index >= len(m.venvs) {
        return fmt.Errorf("venv index out of range")
    }
    m.selectedVenv = index
    return nil
}

// SelectedVenv returns the currently selected venv and a bool indicating
// whether a selection exists.
func (m *AppModel) SelectedVenv() (string, bool) {
    if m.selectedVenv < 0 || m.selectedVenv >= len(m.venvs) {
        return "", false
    }
    return m.venvs[m.selectedVenv], true
}

// SetStatus sets the visible status/message shown in the status area.
func (m *AppModel) SetStatus(s string) {
    m.status = s
}

// ClearStatus clears the status area.
func (m *AppModel) ClearStatus() {
    m.status = ""
}

// SetError records the last error; consumers may choose to display it.
func (m *AppModel) SetError(err error) {
    m.lastErr = err
}

// ClearError clears the recorded error.
func (m *AppModel) ClearError() {
    m.lastErr = nil
}

// LastError returns the stored last error value (may be nil).
func (m *AppModel) LastError() error {
    return m.lastErr
}

// QuitRequested reports whether the model has requested quitting.
func (m *AppModel) QuitRequested() bool {
    return m.requestedQuit
}

