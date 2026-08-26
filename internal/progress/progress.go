package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// TopicProgress tracks study topics: how many times each topic was studied,
// when it was last reviewed, and its status.
type TopicProgress struct {
	Status       string `json:"estado"` // pendiente | en_proceso | dominado
	Repetitions  int    `json:"repeticiones"`
	LastReviewed string `json:"ultima_revision,omitempty"`
}

type Progress struct {
	Topics map[string]TopicProgress `json:"temas"`
}

func Path(root string) string {
	return filepath.Join(root, "data", "progress.json")
}

func load(root string) (*Progress, error) {
	p := &Progress{Topics: map[string]TopicProgress{}}
	b, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, p); err != nil {
		return nil, fmt.Errorf("progreso corrupto: %w", err)
	}
	if p.Topics == nil {
		p.Topics = map[string]TopicProgress{}
	}
	return p, nil
}

func save(root string, p *Progress) error {
	if p.Topics == nil {
		p.Topics = map[string]TopicProgress{}
	}
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(root), append(b, '\n'), 0644)
}

var validStatuses = map[string]bool{"pendiente": true, "en_proceso": true, "dominado": true}

// Set registers a study event for a topic. Repetitions only grow for
// estados != pendiente; going back to pendiente resets them.
func Set(root, topic, status string) (TopicProgress, error) {
	if !validStatuses[status] {
		return TopicProgress{}, fmt.Errorf("estado inválido %q (usá pendiente|en_proceso|dominado)", status)
	}
	p, err := load(root)
	if err != nil {
		return TopicProgress{}, err
	}
	t := p.Topics[topic]
	t.Status = status
	t.LastReviewed = time.Now().Format(time.RFC3339)
	if status == "pendiente" {
		t.Repetitions = 0
	} else {
		t.Repetitions++
	}
	p.Topics[topic] = t
	if err := save(root, p); err != nil {
		return TopicProgress{}, err
	}
	return t, nil
}

// History returns progress sorted by status priority and recency,
// plus a count of closed sessions per topic keyword match.
func History(root string) ([]struct {
	Topic    string
	Progress TopicProgress
}, error) {
	p, err := load(root)
	if err != nil {
		return nil, err
	}
	rank := map[string]int{"dominado": 0, "en_proceso": 1, "pendiente": 2}
	out := []struct {
		Topic    string
		Progress TopicProgress
	}{}
	topics := make([]string, 0, len(p.Topics))
	for t := range p.Topics {
		topics = append(topics, t)
	}
	sort.Slice(topics, func(i, j int) bool {
		a, b := p.Topics[topics[i]], p.Topics[topics[j]]
		if rank[a.Status] != rank[b.Status] {
			return rank[a.Status] < rank[b.Status]
		}
		return topics[i] < topics[j]
	})
	for _, t := range topics {
		out = append(out, struct {
			Topic    string
			Progress TopicProgress
		}{t, p.Topics[t]})
	}
	return out, nil
}

// Cmd implements `apuntes progreso [set <tema> <estado>]`.
func Cmd(root string, args []string, out io.Writer) error {
	switch {
	case len(args) >= 2 && args[0] == "set":
		status := "en_proceso"
		if len(args) >= 3 {
			status = args[2]
		}
		t, err := Set(root, args[1], status)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s: %s (x%d)\n", args[1], t.Status, t.Repetitions)
		return nil
	case len(args) == 0:
		rows, err := History(root)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			fmt.Fprintln(out, "sin progreso registrado; usá `apuntes progreso set <tema> <estado>`")
			return nil
		}
		for _, r := range rows {
			fmt.Fprintf(out, "%-12s x%-3d %s\n", r.Progress.Status, r.Progress.Repetitions, r.Topic)
		}
		return nil
	default:
		return fmt.Errorf("uso: apuntes progreso [set <tema> <pendiente|en_proceso|dominado>]")
	}
}
