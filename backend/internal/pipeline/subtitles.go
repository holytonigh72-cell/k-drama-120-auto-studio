package pipeline

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Cue struct {
	Start int64
	End   int64
	Text  string
}

var timestamp = regexp.MustCompile(`^(\d{2}):(\d{2}):(\d{2}),(\d{3})$`)

func ParseSRT(path string) ([]Cue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var cues []Cue
	var lines []string
	flush := func() error {
		if len(lines) == 0 {
			return nil
		}
		if len(lines) < 3 {
			return fmt.Errorf("불완전한 자막 블록")
		}
		var a, b string
		for _, line := range lines {
			if strings.Contains(line, " --> ") {
				parts := strings.SplitN(line, " --> ", 2)
				a, b = parts[0], parts[1]
				break
			}
		}
		if a == "" {
			return fmt.Errorf("자막 타임코드 누락")
		}
		start, e := parseTimestamp(a)
		if e != nil {
			return e
		}
		end, e := parseTimestamp(b)
		if e != nil {
			return e
		}
		text := strings.Join(lines[2:], " ")
		if start < 0 || end > 300000 || end <= start {
			return fmt.Errorf("자막 시간 범위 오류: %s", a)
		}
		if len(cues) > 0 && start < cues[len(cues)-1].End {
			return fmt.Errorf("자막 시간이 겹칩니다")
		}
		cues = append(cues, Cue{start, end, text})
		lines = nil
		return nil
	}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
		} else {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return cues, nil
}
func parseTimestamp(s string) (int64, error) {
	m := timestamp.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("잘못된 SRT 시간: %s", s)
	}
	v := make([]int64, 4)
	for i := range v {
		x, e := strconv.ParseInt(m[i+1], 10, 64)
		if e != nil {
			return 0, e
		}
		v[i] = x
	}
	return ((v[0]*60+v[1])*60+v[2])*1000 + v[3], nil
}

