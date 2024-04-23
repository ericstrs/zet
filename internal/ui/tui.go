package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ericstrs/zet"
	"github.com/ericstrs/zet/internal/meta"
	"github.com/ericstrs/zet/internal/storage"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SearchUI struct {
	// app is a reference to the tview application
	app *tview.Application

	// inputField is a UI element for text input, allowing users to enter
	// their search queries. The entered text is used for search operations.
	inputField *tview.InputField

	// list represents a table view in the UI, used to display search
	// results. Each row in the table can correspond to a different zettel
	// title, tag line, or zettel.
	list *tview.Table

	// storage is a pointer to the Storage struct which handles
	// interactions with the database.
	storage *storage.Storage

	// screenWidth holds the width of the screen in characters.
	screenWidth int
}

// NewSearchUI creates and initializes a new SearchUI.
func NewSearchUI(s *storage.Storage, query, zetDir, editor string) *SearchUI {
	sui := &SearchUI{
		app:         tview.NewApplication(),
		inputField:  tview.NewInputField(),
		list:        tview.NewTable(),
		storage:     s,
		screenWidth: 50,
	}

	sui.setupUI(query, zetDir, editor)

	return sui
}

// setupUI configures the UI elements.
func (sui *SearchUI) setupUI(query, zetDir, editor string) {
	sui.globalInput()

	// Update screen width before drawing. This won't affect the current
	// drawing, it sets the width for the next draw operation.
	sui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		sui.screenWidth, _ = screen.Size()
		return false
	})

	t := "Loading all zettels in the background. Begin typing to search, or wait to browse."
	zettels := []storage.Zettel{storage.Zettel{Title: t}}
	go func() {
		zettels, _ = sui.storage.AllZettels(`dir_name DESC`)
		sui.app.QueueUpdateDraw(func() {
			text := sui.inputField.GetText()
			if text == "" {
				sui.displayAll(zettels)
			}
		})
	}()

	sui.inputField.SetLabel("Search: ").
		SetFieldWidth(30)
	sui.ipInput(zetDir, editor, &zettels)

	sui.list.SetBorder(true)
	style := tcell.StyleDefault.Background(tcell.Color107).Foreground(tcell.ColorBlack)
	sui.list.SetSelectedStyle(style)
	sui.listInput(zetDir, editor)

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

// globalInput handles input capture for the application.
func (sui *SearchUI) globalInput() {
	sui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			sui.app.Stop()
		}
		return event
	})
}

// ipInput handles input capture for the inputField.
//
// It interprets the following key bindings and triggers corresponding
// actions:
//
//   - Enter: Sets focus to results list.
//   - Ctrl+Enter: Uses current search query as title for new zettel.
//   - Esc: Exits the search interface.
func (sui *SearchUI) ipInput(zetDir, editor string, zettels *[]storage.Zettel) {
	var debounceTimer *time.Timer
	sui.inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// If ctrl+enter pressed, create and open zettel.
		if event.Modifiers() == 2 && event.Rune() == 10 {
			text := sui.inputField.GetText()
			sui.app.Stop()
			// If current link cannot be found, skip auto-linking
			currLink, err := meta.CurrLink(zetDir)
			if err != nil {
				currLink = ""
			}

			if err := zet.CreateAdd(zetDir, editor, text, "", "", currLink, true); err != nil {
				log.Printf("Failed to add zettel: %v\n", err)
			}
		}
		return event
	})
	sui.inputField.SetChangedFunc(func(text string) {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
			go func() {
				latestText := sui.inputField.GetText()
				if latestText == "" {
					sui.app.QueueUpdateDraw(func() {
						sui.displayAll(*zettels)
					})
					return
				}
				zettels := sui.performSearch(latestText)
				sui.app.QueueUpdateDraw(func() {
					sui.updateList(zettels)
				})
			}()
		})
	}).
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				sui.list.SetSelectable(true, false)
				sui.app.SetFocus(sui.list)
			}
		})
}

func (sui *SearchUI) displayAll(zettels []storage.Zettel) {
	row := 0
	for i := 0; i < len(zettels); i++ {
		z := zettels[i]
		// Add zettel dir and title
		s := `[yellow]` + z.DirName + `[white]` + ` ` + z.Title
		sui.list.SetCell(row, 0, tview.NewTableCell(s).
			SetReference(&z))
		row++
	}
	sui.list.ScrollToBeginning()
}

// performSearch gets result zettels to update the results list.
func (sui *SearchUI) performSearch(query string) []storage.ResultZettel {
	if query == "" {
		return []storage.ResultZettel{}
	}
	zettels, err := sui.storage.SearchZettels(query, `[red]`, `[white]`)
	if err != nil {
		zettels = []storage.ResultZettel{storage.ResultZettel{TitleSnippet: "Incorrect syntax"}}
	}
	return zettels
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
			hashedTags := "    #" + strings.ReplaceAll(z.TagsSnippet, " ", " #")
			list.SetCell(row, 0, tview.NewTableCell(hashedTags).
				SetSelectable(false))
			row++
		}
		list.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
		row++
	}
	sui.list.ScrollToBeginning()
}

// listInput handles input capture for the list.
//
// It interprets the following key bindings and triggers corresponding
// actions:
//
//   - l: Open selected zettel.
//   - H: Move to the top of the visible window.
//   - M: Move to the center of the visible window.
//   - L: Move to bottom of the visible window.
//   - space: Page down
//   - b: Page up
//   - ESC, q: Exits the search interface.
//
// If selection is on first result and 'k' is pressed, set focus on
// input field.
func (sui *SearchUI) listInput(zetDir, editor string) {
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
					fz := filepath.Join(zetDir, z.DirName)
					fp := filepath.Join(fz, z.Name)
					sui.app.Stop()
					if err := runCmd(fz, editor, fp); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to open new zettel: %v", err)
					}
				case *storage.Zettel:
					fz := filepath.Join(zetDir, z.DirName)
					fp := filepath.Join(fz, z.Name)
					sui.app.Stop()
					if err := runCmd(fz, editor, fp); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to open new zettel: %v", err)
					}
				default:
					log.Printf("Table cell doesn't reference storage.ResultZettel or storage.Zettel: %T\n", z)
				}
				return nil
			case 'H': // move to top of the visible window
				row, _ := sui.list.GetOffset()
				sui.list.Select(row, 0)
				return nil
			case 'M': // move to middle of the visible window
				row, _ := sui.list.GetOffset()
				_, _, _, height := sui.list.GetInnerRect()
				sui.list.Select(row+height/2, 0)
				return nil
			case 'L': // move to bottom of the visible window
				row, _ := sui.list.GetOffset()
				_, _, _, height := sui.list.GetInnerRect()
				sui.list.Select(row+height-1, 0)
				return nil
			case 'b': // page up (Ctrl-B)
				return tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone)
			case ' ': // page down
				row, _ := sui.list.GetOffset()
				_, _, _, height := sui.list.GetInnerRect()
				newRow := row + height
				if newRow > sui.list.GetRowCount()-1 {
					newRow = sui.list.GetRowCount() - 1
				}
				sui.list.SetOffset(newRow, 0)
				sui.list.Select(newRow, 0)
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
