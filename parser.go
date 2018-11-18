package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

var (
	zeroTime time.Time
)

// Test data structure
type Test struct {
	Name     string
	Package  string
	Time     time.Time
	Status   string
	Children []*Test
	Message  string
	Elapsed  time.Duration
	Stats    map[string]int

	records []*Record
}

// Record is test in JSON output
type Record struct {
	Test    string
	Time    time.Time
	Action  string
	Package string
	Output  string
	Elapsed float64
}

// Sort tests by time
type byTime []*Record

func (b byTime) Len() int           { return len(b) }
func (b byTime) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byTime) Less(i, j int) bool { return b[i].Time.Before(b[j].Time) }

type key struct {
	pkg  string
	test string
}

// Parse parsers test output in JSON format
func Parse(input io.Reader) (*Test, error) {
	tests, err := firstScan(input)
	if err != nil {
		return nil, err
	}

	return assembleTests(tests)
}

func firstScan(input io.Reader) (map[key]*Test, error) {
	tests := make(map[key]*Test)
	scan := NewScanner(input)
	//	tests := make(map[string]*Test)
	for scan.Scan() {
		r := &Record{}
		if err := json.Unmarshal(scan.Bytes(), r); err != nil {
			return nil, fmt.Errorf("%d: error: %s", scan.LineNum(), err)
		}
		k := key{r.Package, r.Test}
		t, ok := tests[k]
		if !ok {
			t = &Test{}
			tests[k] = t
		}
		t.records = append(t.records, r)
	}

	if err := scan.Err(); err != nil {
		return nil, err
	}

	return tests, nil
}

func assembleTests(tests map[key]*Test) (*Test, error) {
	var root *Test
	for _, t := range tests {
		if err := t.assemble(); err != nil {
			return nil, err
		}

		if t.Name == "" {
			if root != nil {
				return nil, fmt.Errorf("more than one root test")
			}
			root = t
		}
	}

	for _, t := range tests {
		if t != root {
			root.Children = append(root.Children, t)
		}
	}

	for _, t := range root.Children {
		if root.Time.Equal(zeroTime) || t.Time.Before(root.Time) {
			root.Time = t.Time
		}
	}

	return root, nil
}

// Count return number of sub tests (including this test)
func (t *Test) Count() int {
	n := 1
	for _, c := range t.Children {
		n += c.Count()
	}

	return n
}

// Stats returns the number of tests and subtests that have status
func (t *Test) calcStats() {
	if t.Stats != nil {
		return
	}

	stats := map[string]int{
		"fail": 0,
		"pass": 0,
		"skip": 0,
	}

	stats[t.Status]++
	for _, c := range t.Children {
		c.calcStats()
		for key := range stats {
			stats[key] += c.Stats[key]
		}
	}

	t.Stats = stats
}

func (t *Test) assemble() error {
	sort.Sort(byTime(t.records))
	var buf bytes.Buffer
	for _, r := range t.records {
		switch r.Action {
		case "run":
			t.Name = r.Test
			t.Package = r.Package
			t.Time = r.Time
		case "output":
			buf.WriteString(r.Output)
		case "pass", "fail", "skip":
			t.Status = r.Action
			t.Elapsed = time.Duration(r.Elapsed) * time.Millisecond
		default:
			return fmt.Errorf("unknown action - %q", r.Action)
		}
	}

	fmt.Printf("%+v\n", t)

	return nil
}
