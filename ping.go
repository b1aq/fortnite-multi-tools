package main

import (
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

type region struct {
	code string
	name string
	hosts []string
}

// Fortnite matchmaking regions mapped to the nearest AWS regional endpoint.
// This measures TCP connect latency, which is close to (not identical to) in-game ping.
var regions = []region{
	{"NAE", "NA East", []string{"dynamodb.us-east-1.amazonaws.com"}},
	{"NAC", "NA Central", []string{"dynamodb.us-east-2.amazonaws.com"}},
	{"NAW", "NA West", []string{"dynamodb.us-west-1.amazonaws.com"}},
	{"EU", "Europe", []string{"dynamodb.eu-central-1.amazonaws.com"}},
	{"BR", "Brazil", []string{"dynamodb.sa-east-1.amazonaws.com"}},
	{"ASIA", "Asia", []string{"dynamodb.ap-northeast-1.amazonaws.com"}},
	{"ME", "Middle East", []string{"dynamodb.me-south-1.amazonaws.com", "ec2.me-south-1.amazonaws.com", "s3.me-south-1.amazonaws.com", "dynamodb.me-central-1.amazonaws.com"}},
	{"OCE", "Oceania", []string{"dynamodb.ap-southeast-2.amazonaws.com"}},
}

type stat struct {
	r    region
	addr string

	mu     sync.Mutex
	last   float64
	ok     bool
	min    float64
	max    float64
	sum    float64
	prev   float64
	jitter float64
	n      int
	sent   int
	lost   int
}

type snap struct {
	r      region
	last   float64
	ok     bool
	sent   int
	n      int
	avg    float64
	min    float64
	max    float64
	jitter float64
	loss   float64
}

func (s *stat) run(stop <-chan struct{}) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		s.probe()
		select {
		case <-stop:
			return
		case <-t.C:
		}
	}
}

func (s *stat) probe() {
	if s.addr == "" {
		// Try each candidate host until one answers, then stick with it.
		for _, h := range s.r.hosts {
			ips, err := net.LookupHost(h)
			if err != nil || len(ips) == 0 {
				continue
			}
			addr := net.JoinHostPort(ips[0], "443")
			if c, derr := net.DialTimeout("tcp", addr, 2*time.Second); derr == nil {
				c.Close()
				s.addr = addr
				break
			}
		}
		if s.addr == "" {
			s.record(0, false)
			return
		}
	}
	start := time.Now()
	c, err := net.DialTimeout("tcp", s.addr, 2*time.Second)
	if err != nil {
		s.record(0, false)
		return
	}
	ms := float64(time.Since(start).Microseconds()) / 1000
	c.Close()
	s.record(ms, true)
}

func (s *stat) record(ms float64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent++
	if !ok {
		s.lost++
		s.ok = false
		return
	}
	s.ok = true
	s.last = ms
	s.sum += ms
	s.n++
	if s.n == 1 || ms < s.min {
		s.min = ms
	}
	if ms > s.max {
		s.max = ms
	}
	if s.n > 1 {
		s.jitter += (math.Abs(ms-s.prev) - s.jitter) / 16
	}
	s.prev = ms
}

func (s *stat) snapshot() snap {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn := snap{r: s.r, last: s.last, ok: s.ok, sent: s.sent, n: s.n, min: s.min, max: s.max, jitter: s.jitter}
	if s.n > 0 {
		sn.avg = s.sum / float64(s.n)
	}
	if s.sent > 0 {
		sn.loss = float64(s.lost) / float64(s.sent) * 100
	}
	return sn
}

func colorFor(ms float64) string {
	switch {
	case ms < 60:
		return green
	case ms < 120:
		return yellow
	default:
		return red
	}
}

func cell(s string, w int, color string) string {
	return color + fmt.Sprintf("%*s", w, s) + reset
}

func bar(ms float64) string {
	const w = 12
	n := int(ms / 200 * w)
	if n > w {
		n = w
	}
	if n < 1 {
		n = 1
	}
	return colorFor(ms) + strings.Repeat("█", n) + gray + strings.Repeat("░", w-n) + reset
}

func render(stats []*stat) {
	snaps := make([]snap, len(stats))
	best := -1
	bestAvg := math.MaxFloat64
	for i, s := range stats {
		snaps[i] = s.snapshot()
		if snaps[i].n > 0 && snaps[i].avg < bestAvg {
			bestAvg = snaps[i].avg
			best = i
		}
	}

	var b strings.Builder
	b.WriteString("\x1b[H")
	line := func(s string) {
		b.WriteString(s)
		b.WriteString("\x1b[K\n")
	}

	line("")
	line(fmt.Sprintf("  %s%s◆ LIVE PING%s  %sFortnite matchmaking regions%s", bold, magenta, reset, gray, reset))
	line(fmt.Sprintf("  %s%s%s", gray, strings.Repeat("─", 88), reset))
	line(fmt.Sprintf("  %s%-5s %-13s %8s %8s %8s %8s %8s %6s%s", gray, "CODE", "REGION", "PING", "AVG", "MIN", "MAX", "JITTER", "LOSS", reset))

	for i, sn := range snaps {
		pingS, col := "...", gray
		barS := strings.Repeat(" ", 12)
		if sn.sent > 0 {
			if sn.ok {
				pingS = fmt.Sprintf("%.0f ms", sn.last)
				col = colorFor(sn.last)
				barS = bar(sn.last)
			} else {
				pingS = "timeout"
				col = red
			}
		}
		avgS, minS, maxS, jitS := "-", "-", "-", "-"
		if sn.n > 0 {
			avgS = fmt.Sprintf("%.0f ms", sn.avg)
			minS = fmt.Sprintf("%.0f ms", sn.min)
			maxS = fmt.Sprintf("%.0f ms", sn.max)
			jitS = fmt.Sprintf("%.1f ms", sn.jitter)
		}
		lossCol := green
		if sn.loss >= 20 {
			lossCol = red
		} else if sn.loss > 0 {
			lossCol = yellow
		}
		mark := " "
		if i == best {
			mark = yellow + "★" + reset
		}
		line("  " + cyan + fmt.Sprintf("%-5s", sn.r.code) + reset + " " +
			fmt.Sprintf("%-13s", sn.r.name) + " " +
			cell(pingS, 8, col) + " " +
			cell(avgS, 8, "") + " " +
			cell(minS, 8, "") + " " +
			cell(maxS, 8, "") + " " +
			cell(jitS, 8, "") + " " +
			cell(fmt.Sprintf("%.0f%%", sn.loss), 6, lossCol) + "  " +
			barS + " " + mark)
	}

	line("")
	if best >= 0 {
		line(fmt.Sprintf("  %sBest region:%s %s%s%s (%.0f ms avg)", gray, reset, bold, snaps[best].r.name, reset, snaps[best].avg))
	} else {
		line("  " + gray + "Measuring..." + reset)
	}
	line("  " + gray + "TCP latency to nearby AWS endpoints — close to in-game ping, not identical." + reset)
	line("  " + gray + "Press Enter to stop." + reset)
	b.WriteString("\x1b[J")
	fmt.Print(b.String())
}

func livePing() {
	stats := make([]*stat, len(regions))
	stop := make(chan struct{})
	for i, r := range regions {
		stats[i] = &stat{r: r}
		go stats[i].run(stop)
	}

	done := make(chan struct{})
	go func() {
		readLine()
		close(done)
	}()

	fmt.Print("\x1b[2J\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	tick := time.NewTicker(400 * time.Millisecond)
	defer tick.Stop()
	for {
		render(stats)
		select {
		case <-done:
			close(stop)
			return
		case <-tick.C:
		}
	}
}
