package expr

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// scope is what the tests read: variables are text, as a flow keeps them, a list param is
// decoded, and a step is its fields.
func scope(name string) (any, bool) {
	value, ok := map[string]any{
		"login": "ana", "expectedLogin": "ana", "password": "x", "expectedPassword": "y",
		"ok": "true", "exit": "0", "count": "3", "env": "prod", "empty": "",
		"hosts": []any{"web-1", "prod-db", "web-2"},
		"cfg":   map[string]any{"db": map[string]any{"port": json.Number("5432")}},
		"build": map[string]any{"output": "done", "exit": "0", "ok": "true"},
		"api":   map[string]any{"output": `{"status": "ready", "items": [1, 2]}`},
		"list":  map[string]any{"output": "a\n\nb\n c \n"},
	}[name]
	return value, ok
}

func eval(t *testing.T, text string) any {
	t.Helper()
	e, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse(%q): %v", text, err)
	}
	value, err := e.Eval(scope)
	if err != nil {
		t.Fatalf("Eval(%q): %v", text, err)
	}
	return value
}

func TestExpressionsReadAsWritten(t *testing.T) {
	for text, want := range map[string]string{
		`login == expectedLogin and password == expectedPassword`:   "false",
		`login == expectedLogin && !(password == expectedPassword)`: "true",
		`not ok`:                          "false",
		`not login == "bob"`:              "true", // not holds the comparison
		`!ok == false`:                    "true", // ! holds only the value
		`exit == 0`:                       "true",
		`count + 1`:                       "4",
		`count * 2 - 1`:                   "5",
		`count % 2`:                       "1",
		`10 % 4 == 2`:                     "true",
		`7 / 2`:                           "3.5",
		`-count`:                          "-3",
		`env + "-eu"`:                     "prod-eu",
		`env in ["prod", "stg"]`:          "true",
		`"db" in hosts[1]`:                "true",
		`"web-3" not in hosts`:            "true",
		`empty ?? "none"`:                 "none",
		`env ?? "none"`:                   "prod",
		`hosts[0]`:                        "web-1",
		`hosts[-1]`:                       "web-2",
		`hosts[9]`:                        "",
		`cfg.db.port`:                     "5432",
		`cfg.db.port > 1000`:              "true",
		`build.output`:                    "done",
		`build.missing`:                   "",
		`api.output.status`:               "ready",
		`api.output.items[1]`:             "2",
		`len(hosts)`:                      "3",
		`len(env)`:                        "4",
		`upper(env)`:                      "PROD",
		`join(hosts, ", ")`:               "web-1, prod-db, web-2",
		`join(map(hosts, h => upper(h)))`: "WEB-1 PROD-DB WEB-2",
		`filter(hosts, h => startsWith(h, "web"))`: `["web-1","web-2"]`,
		`map(hosts, (h, i) => i)`:                  "[0,1,2]",
		`any(hosts, h => startsWith(h, "prod-"))`:  "true",
		`all(hosts, h => contains(h, "-"))`:        "true",
		`find(hosts, h => endsWith(h, "2"))`:       "web-2",
		`lines(list.output)`:                       `["a","b","c"]`,
		`len(lines(list.output))`:                  "3",
		`first(hosts) + " " + last(hosts)`:         "web-1 web-2",
		`sort([3, 10, 2])`:                         "[2,3,10]",
		`unique(["a", "b", "a"])`:                  `["a","b"]`,
		`keys(cfg)`:                                `["db"]`,
		`json(env)`:                                `"prod"`,
		`{user: login, n: count + 0}`:              `{"n":3,"user":"ana"}`,
		`if(ok, "yes", "no")`:                      "yes",
		`max(1, count, 2)`:                         "3",
		`round(7 / 3, 2)`:                          "2.33",
		`match(build.output, "^do")`:               "true",
		`replace(env, "o", "0")`:                   "pr0d",
		`'single' + "double"`:                      "singledouble",
		`"a\"b"`:                                   `a"b`,
		`1 < 2 or nothing_needed`:                  "true", // or stops at what holds
	} {
		if got := String(eval(t, text)); got != want {
			t.Errorf("%s = %s, want %s", text, got, want)
		}
	}
}

func TestTruthy(t *testing.T) {
	for _, value := range []any{nil, false, "", "false", "0", " 0.0 ", 0.0, []any{}, map[string]any{}} {
		if Truthy(value) {
			t.Errorf("Truthy(%#v) = true", value)
		}
	}
	for _, value := range []any{true, "true", "yes", "1", -1.0, []any{""}, "no"} {
		if !Truthy(value) {
			t.Errorf("Truthy(%#v) = false", value)
		}
	}
}

func TestProblemsAreTold(t *testing.T) {
	for text, want := range map[string]string{
		`login = "ana"`:    "write ==",
		`nope(1)`:          "there is no function nope",
		`len(1, 2)`:        "len takes 1 argument",
		`1 < 2 < 3`:        "one comparison at a time",
		`login ==`:         "a value is missing",
		`"open`:            "has no \" to end it",
		`(1 + 2`:           "a ( needs )",
		`join(hosts ", ")`: "apart by commas",
		`login and`:        "a value is missing",
		``:                 "empty",
		`login login`:      "goes on after its end",
	} {
		_, err := Parse(text)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%q) = %v, want %q", text, err, want)
		}
	}
	for text, want := range map[string]string{
		`prod == env`:   `there is no variable or step prod: write "prod" for a text`,
		`env - 1`:       "- needs numbers",
		`count / 0`:     "divides by 0",
		`env.name`:      "has no fields",
		`map(hosts, 1)`: "map: the second argument is a lambda",
	} {
		e, err := Parse(text)
		if err != nil {
			t.Fatalf("Parse(%q): %v", text, err)
		}
		if _, err := e.Eval(scope); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Eval(%q) = %v, want %q", text, err, want)
		}
	}
}

func TestNamesLeaveLambdaParamsOut(t *testing.T) {
	e, err := Parse(`any(hosts, h => h == build.output and h != env) or cfg.db.port > count`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, path := range e.Names() {
		got = append(got, strings.Join(path, "."))
	}
	want := []string{"hosts", "build.output", "env", "cfg.db.port", "count"}
	if !slices.Equal(got, want) {
		t.Errorf("Names = %v, want %v", got, want)
	}
}

func TestParseRefStopsWhereTheReferenceEnds(t *testing.T) {
	for text, want := range map[string]string{
		"name; echo":         "name",
		"name's car":         "name",
		"build.output.":      "build.output",
		"build.output is":    "build.output",
		"hosts[0] and":       "hosts[0]",
		`join(hosts, ", ")!`: `join(hosts, ", ")`,
		"cfg.db.port/x":      "cfg.db.port",
		"name .field":        "name",
		"len (x)":            "len",
		`join(map(hosts, h => "host " + h), "; ")"`: `join(map(hosts, h => "host " + h), "; ")`,
	} {
		e, n, err := ParseRef(text)
		if err != nil {
			t.Errorf("ParseRef(%q): %v", text, err)
			continue
		}
		if text[:n] != want || e.String() != want {
			t.Errorf("ParseRef(%q) took %q, want %q", text, text[:n], want)
		}
	}
}
