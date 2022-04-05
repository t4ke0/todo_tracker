package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

var progressValue float64

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage %s <todo filename>", os.Args[0])
		os.Exit(1)
	}

	todoFile := os.Args[1]

	tracker := newTracker()

	go func() {
		for {
			<-tracker.C
			fmt.Printf("\033[H\033[J")
			fmt.Printf("progress: %.2f\n", progress(todoFile))
		}
	}()

	if err := tracker.trackModifications(todoFile).Error(); err != nil {
		log.Fatal(err)
	}

}

// Tracker
type Tracker struct {
	timer       *time.Timer
	lastModTime time.Time

	C         chan struct{}
	errorChan chan error
}

func newTracker() *Tracker {
	return &Tracker{
		timer:     time.NewTimer(time.Second * 1),
		C:         make(chan struct{}),
		errorChan: make(chan error),
	}
}

func (t Tracker) Error() error {
	err := <-t.errorChan
	if err != nil {
		return err
	}
	return nil
}

func (t *Tracker) trackModifications(filename string) *Tracker {

	go func() {
		for {
			fileInfo, err := os.Stat(filename)
			if err != nil {
				t.errorChan <- err
				continue
			}

			if !fileInfo.ModTime().Equal(t.lastModTime) {
				t.C <- struct{}{}
				t.lastModTime = fileInfo.ModTime()
			}

			<-t.timer.C
			t.timer.Reset(time.Second * 1)
		}
	}()

	return t

}

func progress(filename string) float64 {
	todos, err := parseTodoFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	var sum, total int
	for _, t := range todos {
		t.CalcTodos(&t, &sum, &total, false)
	}

	// PrintTodos(os.Stdout, todos)

	out := float64(sum) / float64(total) * 100
	if progressValue != out {
		progressValue = out
		fd, _ := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0666)
		defer fd.Close()
		PrintTodos(fd, todos)
	}

	return out
}

// PrintTodos
func PrintTodos(seek io.Writer, todos []Todo) {
	for _, t := range todos {
		recursiveTodoPrint(seek, &t, 0)
		fmt.Fprintf(seek, "\n")
	}
}

func recursiveTodoPrint(seek io.Writer, todo *Todo, level int) {
	if todo == nil {
		return
	}

	space := ""
	for i := 0; i < level; i++ {
		space += " "
	}
	var st string
	if todo.IsDone {
		st = Done.String()
	} else {
		st = Undone.String()
	}
	// format := fmt.Sprintf("%%%ds\n", level)
	fmt.Fprintf(seek, "%s%s%s\n", space, st, todo.Content)
	level += 2
	recursiveTodoPrint(seek, todo.sub, level)
}

// Todo
type Todo struct {
	IsDone   bool
	Content  string
	IsParent bool

	sub *Todo
}

// CalcTodos
func (t Todo) CalcTodos(current *Todo, sum, total *int, restDone bool) {
	if current == nil {
		return
	}

	if current.IsDone || restDone {
		if current.IsParent {
			restDone = true
		}
		current.UpdateStatus(Done)
		*sum++
	}

	*total++

	t.CalcTodos(current.sub, sum, total, restDone)
}

func (t *Todo) UpdateStatus(st TodoStatus) {
	t.IsDone = st == Done
}

// TodoStatus
type TodoStatus int

const (
	Done TodoStatus = iota
	Undone
)

// String
func (ts TodoStatus) String() string {
	switch ts {
	case Done:
		return "- [X]"
	case Undone:
		return "- [ ]"
	default:
		// unreachable.
		return ""
	}
}

// FromString
func (ts *TodoStatus) FromString(s string) {
	switch s {
	case Done.String():
		*ts = Done
	case Undone.String():
		*ts = Undone
	default:
		// unreachable.
		panic("FromString: unreachable")
	}
}

func parseTodoFile(filename string) ([]Todo, error) {

	fd, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	reader := bufio.NewReader(fd)

	todos := []Todo{}

	var lineNumber int
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if string(line) == "" {
			lineNumber++
			continue
		}

		var subTodo bool
		st, content := parseLine(string(line), &subTodo)
		if st < 0 {
			return nil, fmt.Errorf("Failed to parse TODO file [LINE %d]", lineNumber+1)
		}

		var done bool
		switch st {
		case Done:
			done = true
		case Undone:
			done = false
		}

		if subTodo {
			if len(todos) == 0 {
				return nil, fmt.Errorf("Found sub todo without it parent [LINE %d]", lineNumber+1)
			}
			todos[len(todos)-1].IsParent = true
			todos[len(todos)-1].sub = &Todo{
				IsDone:  done,
				Content: content,
			}
			lineNumber++
			continue
		}

		todos = append(todos, Todo{
			IsDone:  done,
			Content: content,
		})
		lineNumber++

	}

	return todos, nil
}

func parseLine(line string, isSubTodo *bool) (TodoStatus, string) {

	var token string

	spaceCounter := 0
	for i, c := range line {
		if token == Done.String() || token == Undone.String() {
			ts := new(TodoStatus)
			ts.FromString(token)
			if spaceCounter != 0 {
				*isSubTodo = true
			}
			return *ts, line[i:]
		}

		if c == 32 && token == "" {
			spaceCounter++
			continue
		}
		token += string(c)
	}

	return -1, ""
}
