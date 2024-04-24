# zet

zet is a command-line tool to organize and manage knowledge.

https://github.com/ericstrs/zet/assets/98285990/8fdcb3f4-278d-47b4-8f09-cb8ef17356c1

## Features

* Quick zettel (individual note) creation and linking.
* TUI Full Text Search (FTS) for zettels.
* Split a given zettel into separate zettels.
* Merge zettels by replacing zettel links with the referenced content.

## Install

The command can be built from source or directly installed:

```
go install github.com/ericstrs/zet/cmd/zet@latest
```

## Documentation

Usage, controls, and other documentation has been embedded into the source code. See the source or run the application with the `help` command.

Global:

|Keys|Description|
|----|-----------|
|<kbd>ESC</kbd>|Exit the program|

Input field:

|Keys|Description|
|----|-----------|
|<kbd>ctrl+Enter</kbd>|Use current input field text as title for new note|

Results list:

|Keys|Description|
|----|-----------|
|<kbd>l</kbd>|Open selected zettel|
|<kbd>H</kbd>|Move to the top of the visible window|
|<kbd>M</kbd>|Move to the middle of the visible window|
|<kbd>L</kbd>|Move to the bottom of the visible window|
|<kbd>space</kbd>|Page down|
|<kbd>b</kbd>|Page up|
|<kbd>ESC, q</kbd>|Exists the search interface|

FTS filters:

* `title: <term>` or `t: <term>`
* `body: <term>` or `b: <term>`
* `tags: <term>` or `#: <term>`
