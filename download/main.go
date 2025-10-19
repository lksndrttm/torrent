package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lksndrttm/torrent/torrent"
)

const (
	padding  = 2
	maxWidth = 80
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

func main() {
	if len(os.Args) != 3 {
		log.Fatal("wrong number of arguments")
	}
	tFilePath := os.Args[1]
	outDir := os.Args[2]

	t, err := torrent.New(tFilePath, outDir)
	if err != nil {
		log.Fatal(err)
	}

	m := model{
		progress: progress.New(progress.WithDefaultGradient()),
		Torrent:  t,
	}
	t.Start()

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Oh no!", err)
		os.Exit(1)
	}
}

type tickMsg time.Time

type model struct {
	progress progress.Model
	Torrent  *torrent.Torrent
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case tickMsg:
		if m.progress.Percent() == 1.0 {
			return m, tea.Quit
		}

		currentProgress := float64(m.Torrent.Downloaded()) / float64(m.Torrent.Length())
		cmd := m.progress.SetPercent(currentProgress)
		return m, tea.Batch(tickCmd(), cmd)

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
}

func (m model) View() string {
	downloadingSpeed := m.Torrent.DownloadingSpeed()

	speedStr := ""
	switch {
	case downloadingSpeed > 999999:
		speedStr = fmt.Sprintf("[%d MB/s]", downloadingSpeed/1000000)
	case downloadingSpeed > 999:
		speedStr = fmt.Sprintf("[%d KB/s]", downloadingSpeed/1000)
	default:
		speedStr = fmt.Sprintf("[%d B/s]", downloadingSpeed)
	}

	pad := strings.Repeat(" ", padding)
	return "\n" +
		pad + m.progress.View() + speedStr + "\n\n" +
		pad + helpStyle("Press any key to quit")
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
