package meta

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/storage"
	"github.com/rivo/tview"
)

var listUsage = `NAME

	list - lists all the zettels.

USAGE

  zet ls|list 						  - Prints all zettels to stdout.
	zet ls|list i|interactive - Iteratively browse all zettels.
	zet ls|list help          - Provides command information.`

// ListCmd parses and validates user arguments for the list command.
// If arguments are valid, it calls the desired operation.
func ListCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // no args
		l, err := List(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
		}
		for _, z := range l {
			fmt.Println(z)
		}
	case 3: // one arg
		switch strings.ToLower(args[2]) {
		case `help`:
			fmt.Println(listUsage)
			return nil
		case `interactive`, `i`:
			if err := ListInteractive(c.ZetDir, c.Editor); err != nil {
				return err
			}
		default:
			fmt.Fprintln(os.Stderr, "Error: incorrect sub-command.")
			fmt.Fprintf(os.Stderr, listUsage)
			os.Exit(1)
		}
	}
	return nil
}

// List retrieves a list of zettels and displays them. It opens the
// selected zettel.
func ListInteractive(zetPath, editor string) error {
	l, err := List(zetPath)
	if err != nil {
		return fmt.Errorf("Failed to retrieve the list of zettels: %v", err)
	}
	app := tview.NewApplication()
	list := tview.NewList()
	list.ShowSecondaryText(false)
	for _, z := range l {
		list.AddItem(z, "", 0, nil)
	}
	listInput(app, list, zetPath, editor)

	// Open the selected zettel.
	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		app.Stop()
		s := strings.SplitN(mainText, " ", 2)
		fp := filepath.Join(zetPath, s[0])
		fp = filepath.Join(fp, s[1])
		if err := runCmd(zetPath, editor, fp); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open new zettel: %v", err)
		}
	})

	emptyLine := tview.NewBox()
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).     // Stack vertically.
		AddItem(emptyLine, 1, 1, false). // Add the empty line with a fixed height.
		AddItem(list, 0, 1, true)

	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
	return nil
}

// listInput handles input capture for the tview.List primitive.
func listInput(app *tview.Application, list *tview.List, zetPath, editor string) {
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		default:
			switch event.Rune() {
			case 'l': // open zettel
				app.Stop()
				main, _ := list.GetItemText(list.GetCurrentItem())
				s := strings.SplitN(main, " ", 2)
				fp := filepath.Join(zetPath, s[0])
				fp = filepath.Join(fp, s[1])
				if err := runCmd(zetPath, editor, fp); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to open new zettel: %v", err)
				}
				return nil
			case 'j': // move down
				list.SetCurrentItem(list.GetCurrentItem() + 1)
				return nil
			case 'k': // move up
				list.SetCurrentItem(list.GetCurrentItem() - 1)
				return nil
			case 'G': // go to bottom of list
				list.SetCurrentItem(list.GetItemCount() - 1)
				return nil
			case 'g': // go to top of list
				list.SetCurrentItem(0)
				return nil
			case 'q': // quit app
				app.Stop()
				return nil
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

// List retrieves a list of zettels. It synchronizes the database and
// gets list of zettels.
func List(zetPath string) ([]string, error) {
	var l []string
	s, err := storage.UpdateDB(zetPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to sync database: %v", err)
	}
	defer s.Close()
	files, err := s.Zettels()

	for _, f := range files {
		l = append(l, f.DirName+" "+f.Name)
	}
	return l, nil
}
