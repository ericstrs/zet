package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ericstrs/zet"
	"github.com/ericstrs/zet/internal/meta"
	"github.com/ericstrs/zet/internal/storage"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	syncStateRunning int32 = iota
	syncStateDone
	syncStateFailed
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

	// status displays background sync state without changing the active list.
	status *tview.TextView

	// storage is a pointer to the Storage struct which handles
	// interactions with the database.
	storage *storage.Storage

	// screenWidth holds the width of the screen in characters.
	screenWidth int

	syncState atomic.Int32
}

// NewSearchUI creates and initializes a new SearchUI.
func NewSearchUI(s *storage.Storage, query, zetDir, dbPath, editor string) *SearchUI {
	sui := &SearchUI{
		app:         tview.NewApplication(),
		inputField:  tview.NewInputField(),
		list:        tview.NewTable(),
		status:      tview.NewTextView(),
		storage:     s,
		screenWidth: 50,
	}

	sui.setupUI(query, zetDir, dbPath, editor)

	return sui
}

// setupUI configures the UI elements.
func (sui *SearchUI) setupUI(query, zetDir, dbPath, editor string) {
	sui.globalInput()

	// Update screen width before drawing. This won't affect the current
	// drawing, it sets the width for the next draw operation.
	sui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		sui.screenWidth, _ = screen.Size()
		return false
	})

	sui.inputField.SetLabel("Search: ").
		SetFieldWidth(30)
	if query != "" {
		sui.inputField.SetText(query)
	}
	sui.ipInput(zetDir, editor)

	sui.status.SetTextColor(tcell.ColorGray)
	sui.setStatus("syncing...")

	sui.list.SetBorder(true)
	style := tcell.StyleDefault.Background(tcell.Color107).Foreground(tcell.ColorBlack)
	sui.list.SetSelectedStyle(style)
	sui.listInput(zetDir, editor)

	sui.list.SetCellSimple(0, 0, "Loading cached notes.")
	sui.loadInitialView(query, zetDir, dbPath)

	// Create a Flex layout to position the input field, status, and list.
	topBar := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(sui.inputField, 39, 0, true).
		AddItem(sui.status, 32, 0, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topBar, 1, 0, true).
		AddItem(sui.list, 0, 1, false)

	sui.app.SetRoot(flex, true)
}

// globalInput handles input capture for the application.
func (sui *SearchUI) globalInput() {
	sui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			sui.app.Stop()
		case tcell.KeyCtrlR:
			sui.refreshCurrentView()
			return nil
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
//   - Ctrl+R: Refreshes the current view from the latest database snapshot.
//   - Esc: Exits the search interface.
func (sui *SearchUI) ipInput(zetDir, editor string) {
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
			sui.loadView(text, true)
		})
	}).
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				sui.list.SetSelectable(true, false)
				sui.app.SetFocus(sui.list)
			}
		})
}

func (sui *SearchUI) loadInitialView(query, zetDir, dbPath string) {
	go func() {
		if query == "" {
			zettels, err := sui.storage.ZettelSummaries(`dir_name DESC`)
			sui.app.QueueUpdateDraw(func() {
				if sui.inputField.GetText() != query {
					return
				}
				if err != nil {
					sui.displayMessage(fmt.Sprintf("Error loading cached notes: %v", err))
					return
				}
				sui.displayAll(zettels)
			})
			sui.startBackgroundSync(zetDir, dbPath)
			return
		}

		zettels := sui.performSearch(query)
		sui.app.QueueUpdateDraw(func() {
			if sui.inputField.GetText() != query {
				return
			}
			sui.updateList(zettels)
		})
		sui.startBackgroundSync(zetDir, dbPath)
	}()
}

func (sui *SearchUI) loadView(query string, userInitiated bool) {
	go func() {
		syncDoneAtStart := sui.syncState.Load() == syncStateDone
		if query == "" {
			zettels, err := sui.storage.ZettelSummaries(`dir_name DESC`)
			sui.app.QueueUpdateDraw(func() {
				if sui.inputField.GetText() != query {
					return
				}
				if err != nil {
					sui.displayMessage(fmt.Sprintf("Error loading cached notes: %v", err))
					return
				}
				sui.displayAll(zettels)
				if userInitiated {
					sui.setStatusAfterRefresh(syncDoneAtStart)
				}
			})
			return
		}

		zettels := sui.performSearch(query)
		sui.app.QueueUpdateDraw(func() {
			if sui.inputField.GetText() != query {
				return
			}
			sui.updateList(zettels)
			if userInitiated {
				sui.setStatusAfterRefresh(syncDoneAtStart)
			}
		})
	}()
}

func (sui *SearchUI) refreshCurrentView() {
	sui.setStatus("refreshing...")
	sui.loadView(sui.inputField.GetText(), true)
}

func (sui *SearchUI) startBackgroundSync(zetDir, dbPath string) {
	sui.syncState.Store(syncStateRunning)
	go func() {
		s, err := storage.UpdateDB(zetDir, dbPath)
		if s != nil {
			s.Close()
		}
		if err != nil {
			sui.syncState.Store(syncStateFailed)
			sui.app.QueueUpdateDraw(func() {
				sui.setStatus("sync failed: " + shortStatusError(err))
			})
			return
		}

		sui.syncState.Store(syncStateDone)
		sui.app.QueueUpdateDraw(func() {
			if sui.inputField.HasFocus() {
				sui.refreshCurrentView()
				return
			}
			sui.setStatus("fresh data available")
		})
	}()
}

func (sui *SearchUI) setStatusAfterRefresh(syncDoneAtStart bool) {
	switch sui.syncState.Load() {
	case syncStateDone:
		if syncDoneAtStart {
			sui.setStatus("fresh")
		} else {
			sui.setStatus("fresh data available")
		}
	case syncStateFailed:
		// Keep the failure visible; the current view still came from the last
		// successful database snapshot.
	default:
		sui.setStatus("syncing...")
	}
}

func (sui *SearchUI) setStatus(text string) {
	sui.status.SetText(text)
}

func shortStatusError(err error) string {
	msg := err.Error()
	if len(msg) > 19 {
		return msg[:16] + "..."
	}
	return msg
}

func (sui *SearchUI) displayMessage(msg string) {
	sui.list.Clear()
	sui.list.SetCellSimple(0, 0, msg)
}

func (sui *SearchUI) displayAll(zettels []storage.Zettel) {
	sui.list.Clear()
	if len(zettels) == 0 {
		sui.list.SetCellSimple(0, 0, "No cached notes.")
		return
	}
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
//   - c: Open selected zettel in a newly created tmux window.
//   - r: Refresh current view from the latest database snapshot.
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
			case 'c': // open selected zettel in new tmux window
				// check if tmux is available
				_, err := exec.LookPath("tmux")
				if err != nil {
					return nil
				}

				// check if the current process is running a tmux session
				_, inSession := os.LookupEnv("TMUX")
				if !inSession {
					return nil
				}

				row, col := sui.list.GetSelection()
				cell := sui.list.GetCell(row, col)
				switch z := cell.GetReference().(type) {
				case *storage.ResultZettel:
					fz := filepath.Join(zetDir, z.DirName)
					fp := filepath.Join(fz, z.Name)
					err := runCmd(fz, "tmux", "new-window", "-d", fmt.Sprintf("$SHELL -c '%s %s && $SHELL'", editor, fp))
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create new tmux window and open zettel: %v", err)
					}
				case *storage.Zettel:
					fz := filepath.Join(zetDir, z.DirName)
					fp := filepath.Join(fz, z.Name)
					err := runCmd(fz, "tmux", "new-window", "-d", fmt.Sprintf("$SHELL -c '%s %s && $SHELL'", editor, fp))
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to create new tmux window and open zettel: %v", err)
					}
				default:
					log.Printf("Table cell doesn't reference storage.ResultZettel or storage.Zettel: %T\n", z)
				}
			case 'r': // refresh current view
				sui.refreshCurrentView()
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
