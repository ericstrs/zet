package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/iuiq/zet"
	"github.com/iuiq/zet/internal/storage"
	"github.com/rivo/tview"
)

type SearchUI struct {
	app         *tview.Application
	inputField  *tview.InputField
	list        *tview.Table
	storage     *storage.Storage
	screenWidth int
}

// NewSearchUI creates and initializes a new SearchUI.
func NewSearchUI(s *storage.Storage, query, zetPath, editor string) *SearchUI {
	sui := &SearchUI{
		app:         tview.NewApplication(),
		inputField:  tview.NewInputField(),
		list:        tview.NewTable(),
		storage:     s,
		screenWidth: 50,
	}

	sui.setupUI(query, zetPath, editor)

	return sui
}

// setupUI configures the UI elements.
func (sui *SearchUI) setupUI(query, zetPath, editor string) {
	sui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			sui.app.Stop()
		}
		return event
	})
	// Update screen width before drawing. This won't affect the current
	// drawing, it sets the width for the next draw operation.
	sui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, _ := screen.Size()
		sui.screenWidth = width
		return false
	})
	zettels, _ := sui.storage.AllZettels("")

	sui.inputField.SetLabel("Search: ").
		SetFieldWidth(30).
		SetChangedFunc(func(text string) {
			if text == "" {
				sui.displayAll(zettels)
				return
			}
			sui.performSearch(text)
		}).
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				text := sui.inputField.GetText()
				sui.performSearch(text)
				sui.list.SetSelectable(true, false)
				sui.app.SetFocus(sui.list)
			}
		})
	sui.inputField.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		// If ctrl+enter pressed, create and open zettel.
		if e.Modifiers() == 2 && e.Rune() == 10 {
			text := sui.inputField.GetText()
			sui.app.Stop()
			if err := zet.Add(zetPath, editor, text, "", "", true); err != nil {
				log.Printf("Failed to add zettel: %v", err)
			}
		}
		return e
	})

	sui.list.SetBorder(true)
	sui.listInput(zetPath, editor)
	switch query {
	case "":
		sui.displayAll(zettels)
	default:
		sui.inputField.SetText(query)
	}

	// Create a Flex layout to position the inputField and list
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(sui.inputField, 1, 0, true).
		AddItem(sui.list, 0, 1, false)

	sui.app.SetRoot(flex, true)
}

func (sui *SearchUI) displayAll(zettels []storage.Zettel) {
	row := 0
	for i := 0; i < len(zettels); i++ {
		z := &zettels[i]
		// Add zettel dir and title
		s := `[yellow]` + z.DirName + `[white]` + ` ` + z.Title
		sui.list.SetCell(row, 0, tview.NewTableCell(s).
			SetReference(&z))
		row++
	}
}

// performSearch gets result zettels to update the results list.
func (sui *SearchUI) performSearch(query string) {
	if query == "" {
		return
	}
	start := `[red]`
	end := `[white]`
	zettels, err := sui.storage.SearchZettels(query, start, end)
	if err != nil {
		zettels = []storage.ResultZettel{storage.ResultZettel{TitleSnippet: "Incorrect syntax"}}
	}
	sui.updateList(zettels)
}

// updateList updates the results list with a given slice of zettels.
func (sui *SearchUI) updateList(zettels []storage.ResultZettel) {
	list := sui.list
	list.Clear()
	if len(zettels) == 0 {
		list.SetCellSimple(0, 0, "No matches found.")
		return
	}
	row := 0
	for i := 0; i < len(zettels); i++ {
		z := zettels[i]
		// Add zettel dir and title
		s := `[yellow]` + z.DirName + `[white]` + ` ` + z.TitleSnippet
		list.SetCell(row, 0, tview.NewTableCell(s).
			SetReference(&z))
		row++
		// Add body snippet
		if z.BodySnippet != "" {
			lines := tview.WordWrap(z.BodySnippet, sui.screenWidth)
			for _, line := range lines {
				if line == "" {
					continue
				}
				list.SetCell(row, 0, tview.NewTableCell(line).
					SetSelectable(false))
				row++
			}
		}
		// Add tags snippet
		if z.TagsSnippet != "" {
			hashedTags := "\t\t#" + strings.ReplaceAll(z.TagsSnippet, " ", " #")
			list.SetCell(row, 0, tview.NewTableCell(hashedTags).
				SetSelectable(false))
			row++
		}
	}
}

// listInput handles input capture for the list.
func (sui *SearchUI) listInput(zetPath, editor string) {
	sui.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			sui.app.Stop()
		default:
			switch event.Rune() {
			case 'l': // open zettel
				row, col := sui.list.GetSelection()
				cell := sui.list.GetCell(row, col)
				switch z := cell.GetReference().(type) {
				case *storage.ResultZettel:
					fp := filepath.Join(zetPath, z.DirName)
					fp = filepath.Join(fp, z.Name)
					sui.app.Stop()
					if err := runCmd(zetPath, editor, fp); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to open new zettel: %v", err)
					}
				case *storage.Zettel:
					fp := filepath.Join(zetPath, z.DirName)
					fp = filepath.Join(fp, z.Name)
					sui.app.Stop()
					if err := runCmd(zetPath, editor, fp); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to open new zettel: %v", err)
					}
				default:
					log.Println("Table cell doesn't reference storage.ResultZettel or storage.Zettel.")
				}

				return nil
			case 'q': // quit app
				sui.app.Stop()
			case 'k':
				row, _ := sui.list.GetSelection()
				if row == 0 {
					sui.list.SetSelectable(false, false)
					sui.app.SetFocus(sui.inputField)
				}
			}
		}

		return event
	})
}

// runCmd runs an external command given the path to directory command
// should be executed in, path to command, and command arguments.
func runCmd(execPath, cmdPath string, args ...string) error {
	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = execPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Run starts the TUI application.
func (sui *SearchUI) Run() error {
	return sui.app.Run()
}
