package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio"
)

const (
	minGap  = time.Second * 4
	maxRand = time.Second * 5
)

func main() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer rpio.Close()

	trigger := rpio.Pin(22)
	trigger.Output()

	g := &game{
		players: []*player{
			newPlayer("Green", rpio.Pin(17), rpio.Pin(4)),
			newPlayer("Red", rpio.Pin(10), rpio.Pin(9)),
		},
		triggers: make(chan struct{}),
	}
	go g.run()

	rand.Seed(time.Now().UnixNano())
	for {
		trigger.Low()
		f := rand.Float64() * float64(maxRand)
		d := time.Duration(f) + minGap
		time.Sleep(d)
		g.triggers <- struct{}{}
		trigger.High()
		time.Sleep(time.Second)
		trigger.Low()
	}
}

type player struct {
	name               string
	out                rpio.Pin
	in                 rpio.Pin
	pressed, responded bool
	scores             []time.Duration
	scoreIndex         int
}

func newPlayer(name string, out, in rpio.Pin) *player {
	p := &player{
		name:   name,
		out:    out,
		in:     in,
		scores: make([]time.Duration, 10),
	}
	p.out.Output()
	p.out.Low()
	p.in.Input()
	p.in.PullDown()
	return p
}

func (p *player) stateChanged() bool {
	state := p.in.Read()
	newPressed := state == rpio.High
	changed := p.pressed != newPressed
	p.pressed = newPressed
	return changed
}

func (p *player) reset() {
	// if we didn't respond, then forfit
	if !p.responded {
		p.score(time.Second * 2)
	}
	p.responded = false
}

func (p *player) score(d time.Duration) {
	p.responded = true
	p.scores[p.scoreIndex] = d
	p.scoreIndex++
	if p.scoreIndex >= 10 {
		p.scoreIndex = 0
	}
}

func (p *player) total() int64 {
	var t int64
	for _, s := range p.scores {
		t += int64(s)
	}
	return t
}

type game struct {
	players  []*player
	triggers chan struct{}
}

func (g *game) run() {
	timer := time.NewTicker(time.Millisecond * 50)
	lastTrigger := time.Time{}
	for {
		select {
		case <-timer.C:
			for i, p := range g.players {
				if !p.stateChanged() {
					continue
				}
				fmt.Printf("Got state change for player %v\n", i+1)
				if p.pressed && !p.responded && !lastTrigger.IsZero() {
					p.score(time.Now().Sub(lastTrigger))
					g.pickWinner()
				}
			}
		case <-g.triggers:
			fmt.Println("Got a trigger")
			lastTrigger = time.Now()
			for _, p := range g.players {
				p.reset()
			}
			g.pickWinner()
		}
	}

}

func (g *game) pickWinner() {
	var winner *player
	for _, p := range g.players {
		p.out.Low()
		if winner == nil || winner.total() > p.total() { // lowest wins
			winner = p
		}
	}
	fmt.Printf("Winner %s has %v", winner.name, winner.total())
	winner.out.High()
}

