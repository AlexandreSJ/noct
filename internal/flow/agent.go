package flow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// An agent step asks an AI agent the user has, such as claude, ollama or opencode, and keeps its
// answer for the steps after it: {{.answer}}, or {{.steps.<id>.answer}}. With choices, the answer
// is the first of them it says, and an answer that says none fails the step, so an if can branch
// on it. The question goes to the agent's command as its input, or in place of {prompt}, never
// through a shell.

// Agent is what an agent step asks: Ask, a template; Use, the agent, or "" for the default one;
// Choices, the answers it may give.
type Agent struct {
	Ask     string   `yaml:"ask"`
	Use     string   `yaml:"use,omitempty"`
	Choices []string `yaml:"choices,omitempty"`
}

// promptPlaceholder in an agent's command is where the question goes, as one argument.
const promptPlaceholder = "{prompt}"

// AgentArgv is an agent's command as arguments, with the question in place of {prompt}, or, when
// the command has no {prompt}, as the input it reads.
func AgentArgv(command, question string) (argv []string, input string, err error) {
	argv, err = splitCommand(command)
	if err != nil || len(argv) == 0 {
		return nil, "", cmp.Or(err, errors.New("the agent's command is empty"))
	}
	placed := false
	for i, arg := range argv {
		if arg == promptPlaceholder {
			argv[i], placed = question, true
		}
	}
	if !placed {
		input = question
	}
	return argv, input, nil
}

// splitCommand splits a command line into words as a shell would for plain quoting: spaces part
// words, and '…' or "…" keep spaces and may be empty.
func splitCommand(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord, quote := false, rune(0)
	for _, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			word.WriteRune(r)
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words, inWord = append(words, word.String()), false
				word.Reset()
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("a quote is not closed in %q", line)
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// thinking is what models that reason aloud print before their answer: ollama's
// Thinking... ...done thinking. and the <think> tags of others.
var thinking = regexp.MustCompile(`(?s)Thinking\.\.\..*?\.\.\.done thinking\.|<think>.*?</think>`)

func withoutThinking(output string) string {
	return thinking.ReplaceAllString(output, "")
}

// pickChoice is the first of choices the answer says, as a word in any case, or "".
func pickChoice(answer string, choices []string) string {
	words := strings.FieldsFunc(strings.ToLower(answer), func(r rune) bool {
		return !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	for _, word := range words {
		for _, choice := range choices {
			if strings.ToLower(choice) == word {
				return choice
			}
		}
	}
	return ""
}

func (x *running) runAgent(s Step, path string) bool {
	question, err := x.render(s.Agent.Ask)
	if err != nil {
		return x.stop(path, "ask: "+err.Error(), 1)
	}
	if len(s.Agent.Choices) > 0 {
		question += "\n\nAnswer with one word, exactly one of: " + strings.Join(s.Agent.Choices, ", ") + "."
	}
	use, err := x.render(s.Agent.Use)
	if err != nil {
		return x.stop(path, "use: "+err.Error(), 1)
	}
	name, command, err := ResolveAgent(x.Agents, cmp.Or(strings.TrimSpace(use), x.DefaultAgent, AutoAgent))
	if err != nil {
		return x.stop(path, err.Error(), 1)
	}
	argv, input, err := AgentArgv(command, question)
	if err != nil {
		return x.stop(path, "agent "+name+": "+err.Error(), 1)
	}
	x.header("◆", s.Label(), "ask "+name)
	start := time.Now()
	deadline := x.stepDeadline(s)
	output, exit := x.ask(argv, input, deadline)
	timedOut := exit != 0 && passed(deadline)
	if timedOut {
		exit = timeoutExit
	}
	answer := strings.TrimSpace(withoutThinking(output))
	captures := capture(x.capturePatterns(s), output)
	if captures == nil {
		captures = map[string]string{}
	}
	captures["answer"] = answer
	chose := len(s.Agent.Choices) == 0
	if !chose {
		captures["answer"] = pickChoice(answer, s.Agent.Choices)
		chose = captures["answer"] != ""
	}
	result := StepResult{Status: StatusOK, Exit: exit, Duration: time.Since(start), Captures: captures, Output: truncate(output, x.KeepOutput)}
	ok := !timedOut && chose && succeeded(s.Expect, exit, output)
	x.keepStepValues(s.ID, result, output, ok)
	if ok {
		x.result.Steps[path] = result
		return true
	}
	result.Status = StatusFailed
	switch {
	case timedOut:
		result.Message = "timed out after " + s.Timeout
	case exit == 0 && !chose:
		result.Message = fmt.Sprintf("the agent did not answer one of %s", strings.Join(s.Agent.Choices, ", "))
	default:
		result.Message = failure(s.Expect, exit)
	}
	x.result.Steps[path] = result
	if x.runTimedOut() {
		return x.stopTimedOut(path)
	}
	return x.afterFailure(s, path, result.Message, max(exit, 1))
}

// ask runs the agent's command with the question as its input, showing what it says as it comes,
// and stops it at the deadline.
func (x *running) ask(argv []string, input string, deadline time.Time) (string, int) {
	ctx, cancel := context.Background(), context.CancelFunc(func() {})
	if !deadline.IsZero() {
		ctx, cancel = context.WithDeadline(ctx, deadline)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	stopGently(cmd, nil)
	cmd.Dir, cmd.Stdin = x.Dir, strings.NewReader(input)
	var kept limitedBuffer
	out := &lockedWriter{w: x.Out} // the answer and the agent's messages come at once
	cmd.Stdout = io.MultiWriter(out, &kept)
	cmd.Stderr = out
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err != nil && passed(deadline):
		return kept.text(), timeoutExit
	case errors.As(err, &exit):
		return kept.text(), exit.ExitCode()
	case err != nil:
		fmt.Fprintln(x.Out, err)
		return kept.text(), 127
	}
	return kept.text(), 0
}

// lockedWriter lets two of a command's outputs write to one writer.
type lockedWriter struct {
	lock sync.Mutex
	w    io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.w.Write(p)
}
