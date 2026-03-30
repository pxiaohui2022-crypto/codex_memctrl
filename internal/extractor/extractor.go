package extractor

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/memory"
)

var (
	splitPattern        = regexp.MustCompile(`[\n\r;；!?！？]+`)
	spacePattern        = regexp.MustCompile(`\s+`)
	pathPattern         = regexp.MustCompile(`(?:[A-Za-z]:\\|/)[^\s]+|(?:[\w.-]+/)+[\w.-]+`)
	commandPattern      = regexp.MustCompile(`\b(go|npm|pnpm|yarn|python|pytest|cargo|make|cmake|docker|uv|poetry|salome|codeaster|bash|sh)\b`)
	profilePattern      = regexp.MustCompile(`(?i)(prefer|concise|brief|detailed|respond|reply|output|style|language|中文|英文|简洁|详细|回复|输出风格)`)
	providerNotePattern = regexp.MustCompile(`(?i)(codex|claude|openai|provider|api|session|memory|服务商|会话|模型|兼容)`)
	todoPattern         = regexp.MustCompile(`(?i)(todo|follow up|later|next step|remember to|待办|后续|下一步|记得)`)
	constraintPattern   = regexp.MustCompile(`(?i)(must|should|prefer|default|do not|don't|never|always|不要|必须|应该|默认|不允许|要依据|先运行|先.*再)`)
	actionPattern       = regexp.MustCompile(`(?i)^(use|prefer|keep|avoid|check|review|run|launch|set|ensure|verify|remember|fix|follow)\b|^(用|检查|确认|保持|避免|先)`)
	noisePattern        = regexp.MustCompile(`(?i)^(traceback|file\s+".*", line \d+|copying |exit_code|<info>|<stderr>|<stdout>|\[?\d+,\d+\]?<stdout>|\[?\d+,\d+\]?<stderr>)`)
)

type Item struct {
	Text      string
	Source    memory.Source
	Timestamp time.Time
}

type Options struct {
	Workspace     string
	Repo          string
	Provider      string
	Status        string
	Global        bool
	Max           int
	MinConfidence float64
}

func Extract(items []Item, opts Options) ([]memory.Memory, error) {
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = memory.StatusCandidate
	}
	if opts.MinConfidence <= 0 {
		opts.MinConfidence = 0.65
	}
	if opts.Max <= 0 {
		opts.Max = 20
	}

	out := make([]memory.Memory, 0, min(len(items), opts.Max))
	seen := make(map[string]struct{})
	for _, item := range items {
		segments := splitSegments(item.Text)
		for index, segment := range segments {
			if len(out) >= opts.Max {
				return out, nil
			}
			candidate, ok := classifySegment(segment)
			if !ok || candidate.confidence < opts.MinConfidence {
				continue
			}
			key := candidate.kind + "|" + strings.ToLower(candidate.summary)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			mem, err := memory.New(candidate.kind, candidate.summary)
			if err != nil {
				return nil, err
			}
			mem.Status = status
			mem.Confidence = candidate.confidence
			mem.Details = candidate.details
			mem.Tags = candidate.tags
			mem.Source = item.Source
			if mem.Source.Turn == 0 {
				mem.Source.Turn = index + 1
			}
			if !opts.Global {
				mem.Scope.Workspace = opts.Workspace
				mem.Scope.Repo = opts.Repo
			}
			mem.Scope.Provider = opts.Provider
			mem.ID = extractedID(mem)
			mem.UpdatedAt = chooseTime(item.Timestamp, mem.UpdatedAt)
			mem.Normalize()
			out = append(out, mem)
		}
	}
	return out, nil
}

type candidate struct {
	kind       string
	summary    string
	details    string
	tags       []string
	confidence float64
}

func classifySegment(segment string) (candidate, bool) {
	segment = normalizeSegment(segment)
	if segment == "" || isNoise(segment) {
		return candidate{}, false
	}

	kind := ""
	confidence := 0.0
	switch {
	case profilePattern.MatchString(segment):
		kind = memory.KindProfile
		confidence = 0.82
	case isProjectInstruction(segment):
		kind = memory.KindProject
		confidence = 0.8
	case providerNotePattern.MatchString(segment) && constraintPattern.MatchString(segment):
		kind = memory.KindProviderNote
		confidence = 0.8
	case constraintPattern.MatchString(segment):
		kind = memory.KindDecision
		confidence = 0.78
	case todoPattern.MatchString(segment) || looksLikeTodo(segment):
		kind = memory.KindTodo
		confidence = 0.72
	case actionPattern.MatchString(segment):
		kind = memory.KindDecision
		confidence = 0.72
	case pathPattern.MatchString(segment):
		kind = memory.KindArtifact
		confidence = 0.68
	default:
		return candidate{}, false
	}

	summary := shorten(segment, 96)
	details := ""
	if summary != segment {
		details = segment
	}
	tags := extractTags(kind, segment)
	return candidate{
		kind:       kind,
		summary:    summary,
		details:    details,
		tags:       tags,
		confidence: confidence,
	}, true
}

func splitSegments(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	chunks := splitPattern.Split(text, -1)
	segments := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = normalizeSegment(chunk)
		if chunk == "" {
			continue
		}
		segments = append(segments, chunk)
	}
	return segments
}

func normalizeSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	segment = strings.TrimPrefix(segment, "- ")
	segment = strings.TrimPrefix(segment, "* ")
	segment = strings.TrimPrefix(segment, "> ")
	segment = spacePattern.ReplaceAllString(segment, " ")
	return strings.TrimSpace(segment)
}

func isProjectInstruction(segment string) bool {
	return commandPattern.MatchString(segment) ||
		(strings.Contains(segment, "运行") && (strings.Contains(segment, "先") || strings.Contains(segment, "目录"))) ||
		strings.Contains(strings.ToLower(segment), "workspace") ||
		strings.Contains(strings.ToLower(segment), "repo") ||
		strings.Contains(segment, "目录") ||
		(strings.Contains(segment, "路径") && pathPattern.MatchString(segment))
}

func looksLikeTodo(segment string) bool {
	lower := strings.ToLower(segment)
	return strings.Contains(lower, "before ") ||
		strings.Contains(lower, "after ") ||
		strings.Contains(lower, "next") ||
		strings.Contains(segment, "之前") ||
		strings.Contains(segment, "之后")
}

func isNoise(segment string) bool {
	if noisePattern.MatchString(segment) {
		return true
	}
	if len(segment) > 240 && strings.Count(segment, " ") < 3 && !strings.Contains(segment, "中文") {
		return true
	}
	return false
}

func extractTags(kind, segment string) []string {
	tags := []string{kind}
	lower := strings.ToLower(segment)
	keywords := map[string][]string{
		"codex":     {"codex"},
		"claude":    {"claude"},
		"sqlite":    {"sqlite"},
		"release":   {"release"},
		"pipeline":  {"pipeline"},
		"github":    {"github"},
		"go":        {"go"},
		"python":    {"python"},
		"salome":    {"salome"},
		"codeaster": {"codeaster", "aster"},
		"中文":        {"zh"},
		"english":   {"en"},
	}
	for pattern, values := range keywords {
		if strings.Contains(lower, pattern) || strings.Contains(segment, pattern) {
			tags = append(tags, values...)
		}
	}
	if pathPattern.MatchString(segment) {
		tags = append(tags, "path")
	}
	if commandPattern.MatchString(lower) {
		tags = append(tags, "command")
	}
	return normalizeTags(tags)
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	slices.Sort(out)
	return out
}

func shorten(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func chooseTime(preferred, fallback time.Time) time.Time {
	if preferred.IsZero() {
		return fallback
	}
	return preferred.UTC()
}

func extractedID(mem memory.Memory) string {
	sum := sha1.Sum([]byte(strings.Join([]string{
		mem.Kind,
		strings.ToLower(strings.TrimSpace(mem.Summary)),
		cleanEmpty(mem.Scope.Workspace),
		strings.ToLower(strings.TrimSpace(mem.Scope.Repo)),
		strings.ToLower(strings.TrimSpace(mem.Scope.Provider)),
		strings.ToLower(strings.TrimSpace(mem.Source.Provider)),
		strings.TrimSpace(mem.Source.SessionID),
		fmt.Sprintf("%d", mem.Source.Turn),
	}, "|")))
	return "memx_" + fmt.Sprintf("%x", sum[:6])
}

func cleanEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return filepath.Clean(value)
}

type HistoryEntry struct {
	SessionID string `json:"session_id"`
	TS        int64  `json:"ts"`
	Text      string `json:"text"`
}

type HistorySummary struct {
	Path               string    `json:"path"`
	Exists             bool      `json:"exists"`
	TotalEntries       int       `json:"total_entries"`
	LatestSessionID    string    `json:"latest_session_id,omitempty"`
	LatestSessionTurns int       `json:"latest_session_turns,omitempty"`
	LatestTimestamp    time.Time `json:"latest_timestamp,omitempty"`
}

func DefaultCodexHistoryPath() (string, error) {
	root, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".codex", "history.jsonl"), nil
}

func InspectCodexHistory(path string) (HistorySummary, error) {
	if strings.TrimSpace(path) == "" {
		defaultPath, err := DefaultCodexHistoryPath()
		if err != nil {
			return HistorySummary{}, err
		}
		path = defaultPath
	}

	summary := HistorySummary{Path: path}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return summary, nil
		}
		return HistorySummary{}, err
	}
	defer file.Close()
	summary.Exists = true

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 16*1024*1024)

	sessionTurns := make(map[string]int)
	sessionLastTS := make(map[string]int64)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return HistorySummary{}, err
		}
		if strings.TrimSpace(entry.Text) == "" {
			continue
		}
		summary.TotalEntries++
		summary.LatestSessionID = entry.SessionID
		sessionTurns[entry.SessionID]++
		sessionLastTS[entry.SessionID] = entry.TS
	}
	if err := scanner.Err(); err != nil {
		return HistorySummary{}, err
	}
	if summary.LatestSessionID == "" {
		return summary, nil
	}

	summary.LatestSessionTurns = sessionTurns[summary.LatestSessionID]
	if ts := sessionLastTS[summary.LatestSessionID]; ts > 0 {
		summary.LatestTimestamp = time.Unix(ts, 0).UTC()
	}
	return summary, nil
}

func LoadCodexHistory(path, sessionID string, recentTurns int) ([]Item, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 16*1024*1024)

	entries := make([]HistoryEntry, 0, 128)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(entry.Text) == "" {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	if len(entries) == 0 {
		return nil, "", errors.New("no codex history entries found")
	}

	if sessionID == "" {
		sessionID = entries[len(entries)-1].SessionID
	}

	filtered := make([]HistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.SessionID != sessionID {
			continue
		}
		filtered = append(filtered, entry)
	}
	if len(filtered) == 0 {
		return nil, "", errors.New("no codex history entries found for the requested session")
	}
	if recentTurns > 0 && len(filtered) > recentTurns {
		filtered = filtered[len(filtered)-recentTurns:]
	}

	items := make([]Item, 0, len(filtered))
	for index, entry := range filtered {
		items = append(items, Item{
			Text: entry.Text,
			Source: memory.Source{
				Provider:  "codex",
				SessionID: entry.SessionID,
				Turn:      index + 1,
			},
			Timestamp: time.Unix(entry.TS, 0).UTC(),
		})
	}
	return items, sessionID, nil
}
