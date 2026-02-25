package main

import (
	"context"
	"fmt"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func Loop(prompt string, f func(string), db DatabaseClient) error {
	var history []string
	for {
		query, stop, err := GetInput(prompt, history)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		if query == "exit" {
			return nil
		}

		// Handle special commands
		if query == "\\dt" {
			err := db.ListTables(context.Background())
			if err != nil {
				fmt.Printf("Error listing tables: %v\n", err)
			}
		} else {
			history = append(history, query)
			f(query)
		}
	}
}

func GetInput(prompt string, history []string) (string, bool, error) {
	app := tea.NewProgram(NewInput(prompt, history))
	m, err := app.Run()
	input := m.(*Input)
	return input.textinput.Text, input.stop, err
}

type Input struct {
	textinput  *Textinput
	stop       bool
	history    []string
	historyIdx int
}

func NewInput(prompt string, history []string) *Input {
	model := NewTextInput()
	model.Prompt = prompt + "> "
	model.Width = 30
	return &Input{
		textinput:  model,
		history:    history,
		historyIdx: -1,
	}
}

func (i *Input) Init() tea.Cmd {
	return textinput.Blink
}

func (i *Input) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			return i, tea.Quit
		case "ctrl+q":
			i.stop = true
			return i, tea.Quit
		case "ctrl+c":
			if i.textinput.Value() == "" {
				i.stop = true
				return i, tea.Quit
			} else {
				i.textinput.SetValue("")
				return i, nil
			}
		case "escape":
			i.stop = true
			return i, tea.Quit
		case "up":
			if i.historyIdx == -1 {
				i.historyIdx = len(i.history)
			}
			if i.historyIdx > 0 {
				i.historyIdx--
			}
			if len(i.history) > 0 {
				i.textinput.SetValue(i.history[i.historyIdx])
			}
		case "down":
			switch {
			case i.historyIdx == -1:
			case i.historyIdx < len(i.history)-1:
				i.historyIdx++
				i.textinput.SetValue(i.history[i.historyIdx])
			case i.historyIdx == len(i.history)-1:
				i.historyIdx = -1
				i.textinput.SetValue("")
			}
		default:
		}
	}

	var cmd tea.Cmd
	i.textinput, cmd = i.textinput.Update(msg)
	return i, cmd
}

func (i *Input) View() tea.View {
	return tea.NewView(i.textinput.View() + "\n")
}

var _ tea.Model = &Input{}
