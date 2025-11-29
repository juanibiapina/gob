package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Stop      key.Binding
	Kill      key.Binding
	Restart   key.Binding
	Remove    key.Binding
	New       key.Binding
	ToggleAll key.Binding
	Follow    key.Binding
	Help      key.Binding
	Quit      key.Binding
	Escape    key.Binding
	Tab       key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter", "l"),
		key.WithHelp("enter/l", "view logs"),
	),
	Stop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "stop"),
	),
	Kill: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "force kill"),
	),
	Restart: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "restart"),
	),
	Remove: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	New: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new job"),
	),
	ToggleAll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "toggle all dirs"),
	),
	Follow: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "toggle follow"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc", "h"),
		key.WithHelp("esc/h", "back"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Stop, k.Restart, k.ToggleAll, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Escape},
		{k.Stop, k.Kill, k.Restart, k.Remove},
		{k.New, k.ToggleAll, k.Help, k.Quit},
	}
}
