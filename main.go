package main

import (
    "os"
    "strconv"
    "os/exec"
    "strings"
    "fmt"
    "syscall"
    "github.com/mattn/go-runewidth"
    "github.com/rickdejager/termbox-go"
    "github.com/sinomoe/gosnake"
    "time"
)

type color termbox.Attribute

const (
    cyan      = color(termbox.ColorCyan)
    black     = color(termbox.ColorBlack)
    yellow    = color(termbox.ColorYellow)
    white     = color(termbox.ColorWhite)
    bold      = color(termbox.AttrBold)
    whiteBold = color(termbox.ColorWhite) | bold
)

func drawCell(x, y int, fg, bg color, ch rune) {
    termbox.SetCell(x, y, ch, termbox.Attribute(fg), termbox.Attribute(bg))
}

func tbprint(x, y int, fg, bg color, msg string) {
    for _, c := range msg {
        termbox.SetCell(x, y, c, termbox.Attribute(fg), termbox.Attribute(bg))
        x += runewidth.RuneWidth(c)
    }
}

func clearScreen() {
    if err := termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
        panic(err)
    }
}

func render() {
    if err := termbox.Flush(); err != nil {
        panic(err)
    }
}

type snake struct {
    head, body rune
}

type wall struct {
    corners                  [4]rune
    top, bottom, left, right rune
}

type style struct {
    snake snake
    food  rune
    wall  wall
}

type gameBox struct {
    game       *gosnake.Game
    xOff, yOff int
    style      style
}

func (gb *gameBox) drawSnake() {
    for _, v := range gb.game.World.Snake.Bodies {
        drawCell(v.X, v.Y, cyan, black, gb.style.snake.body)
    }
}

func (gb *gameBox) drawFood() {
    f := gb.game.World.Food
    drawCell(f.X, f.Y, yellow, black, gb.style.food)
}

func (gb *gameBox) drawWall() {
    xOff := gb.xOff
    yOff := gb.yOff
    xLen := gb.game.World.XLen
    yLen := gb.game.World.YLen
    drawCell(xOff, yOff, whiteBold, black, gb.style.wall.corners[0])
    drawCell(xOff+xLen, yOff, whiteBold, black, gb.style.wall.corners[1])
    drawCell(xOff, yOff+yLen, whiteBold, black, gb.style.wall.corners[2])
    drawCell(xOff+xLen, yOff+yLen, whiteBold, black, gb.style.wall.corners[3])
    for i := 1; i < xLen; i++ {
        drawCell(xOff+i, yOff, whiteBold, black, gb.style.wall.top)
        drawCell(xOff+i, yOff+yLen, whiteBold, black, gb.style.wall.bottom)
    }
    for i := 1; i < yLen; i++ {
        drawCell(xOff, yOff+i, whiteBold, black, gb.style.wall.left)
        drawCell(xOff+xLen, yOff+i, whiteBold, black, gb.style.wall.right)
    }
}

func (gb *gameBox) updateAndDrawAll() {
    clearScreen()
    gb.drawWall()
    gb.drawFood()
    gb.drawSnake()
    gb.drawStatus()
    render()
}

func (gb *gameBox) drawStatus() {
    G := gb.game
    tbprint(41, 1, black, white, "How about some snake?")
    tbprint(41, 2, black, white, "Use WASD to play.")
    scoreMsg := fmt.Sprintf("Score: %d", G.Score())
    tbprint(41, 3, black, white, scoreMsg)
    if G.IsOver() {
        tbprint(41, 4, black, white, "game over")
    }
    head := G.World.Snake.Head()
    headMsg := fmt.Sprintf("Head: (%d, %d)", head.X, head.Y)
    tbprint(41, 5, black, white, headMsg)
    food := G.World.Food
    foodMsg := fmt.Sprintf("Food: (%d, %d)", food.X, food.Y)
    tbprint(41, 6, black, white, foodMsg)
    tbprint(41, 19, black, white, "PeTSnake")
    tbprint(41, 20, black, white, "-RdJ")
}


func main() {
    if len(os.Args) < 2 {
        fmt.Println("usage: ./petsnake [/dev/pts/X]")
        return
    }

    dev := os.Args[1]

    // Open a raw syscall to force the output to be sent to us.
    // Golang built-ins are not fast enough / buffer too much.
    fd_read, err := syscall.Open(dev, syscall.O_RDONLY, 0777)

    if err != nil {
        fmt.Println("Failed to open " + dev)
        return
    }
    defer syscall.Close(fd_read)

    ///// Initialize a snake game
    G := gosnake.GameInit(gosnake.GameConfig{
        XLen: 40,
        YLen: 20,
        BabySnake: gosnake.Snake{
            Bodies: []gosnake.Body{{5, 12}, {6, 12}, {7, 12}, {8, 12}},
            Len:    4,
        },
        InitFood:      gosnake.Food{X: 22, Y: 16},
        WallGenerator: gosnake.DefaultWallGenerator,
    })
    gb := &gameBox{
        game: G,
        xOff: 0,
        yOff: 0,
        style: style{
            snake: snake{head: '●', body: '●'},
            food:  '◉',
            wall:  wall{corners: [4]rune{'+', '+', '+', '+'}, top: '-', bottom: '-', left: '|', right: '|'},
        },
    }

    if err := termbox.Init(dev); err != nil {
        panic(err)
    }
    defer termbox.Close()
    termbox.SetInputMode(termbox.InputEsc)

    gb.updateAndDrawAll()

    time.Sleep(3 * time.Second)

    currentKey := 3

    // Main game loop
    go func() {
        for {
            select {
            case <-time.After(300 * time.Millisecond):
                if G.IsOver() {
                    do_gameover(dev)
                }
                switch currentKey {
                case 0:
                    G.WalkUp()
                case 1:
                    G.WalkDown()
                case 2:
                    G.WalkLeft()
                case 3:
                    G.WalkRight()
                }
            }
            gb.updateAndDrawAll()
        }
    }()

    do_read(fd_read, &currentKey)

}

////////////////////////////////////////////////////////////
// PTS Related Stuff
////////////////////////////////////////////////////////////

func do_read(fd int, dst *int) {

    // create a single byte buffer to handle keystrokes
    buf := make([]byte, 1)

    for {
        n, err := syscall.Read(fd, buf)
        if err != nil {
            break
        }

        switch buf[n-1] {
            // w
            case 0x77:
                *dst = 0
            // s
            case 0x73:
                *dst = 1
            // a
            case 0x61:
                *dst = 2
            // d
            case 0x64:
                *dst = 3
        }
    }
}

func do_gameover(dev string) {
    // Try to kill the PTS owner
    out, _ := exec.Command("fuser", dev).Output()

    // kill the lowest PID that uses this FD. (we are the other, higher pid)
    pids := strings.Fields(string(out))
    pid, err := strconv.Atoi(pids[0])

    // Make sure we don't end up killing our own process in some freak accident
    if err == nil && pid != os.Getpid() {
        syscall.Kill(pid, 9)
    }

    os.Exit(0)
}

