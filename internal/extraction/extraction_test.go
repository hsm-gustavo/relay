package extraction

import "testing"

func TestPrimitivePaths(t *testing.T) {
	body := []byte(`{"access_token":"abc123","user":{"id":42},"data":{"user":{"name":"Ada"}},"items":[{"id":9007199254740993}],"enabled":true,"disabled":false,"empty":"","number":1.20e+30,"nothing":null}`)
	for _, tt := range []struct{ path, want string }{
		{"$.access_token", "abc123"}, {"$.user.id", "42"}, {"$.data.user.name", "Ada"}, {"$.items[0].id", "9007199254740993"}, {"$.enabled", "true"}, {"$.disabled", "false"}, {"$.empty", ""}, {"$.number", "1.20e+30"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			result := Evaluate(200, body, true, []Rule{{Path: tt.path, Variable: "value"}})[0]
			if result.Status != Success || result.Value != tt.want {
				t.Fatalf("result = %+v", result)
			}
		})
	}
	for _, path := range []string{"$.missing", "$.user.missing", "$.items[5].id", "$.user[0]", "$.nothing", "$.user", "$.items", "", "user.id", "$..user", "$.items[*]", "$.items[-1]", "$.items[", "$['user']", "$.items[999999999999999999999999]"} {
		t.Run(path, func(t *testing.T) {
			result := Evaluate(200, body, true, []Rule{{Path: path, Variable: "value"}})[0]
			if result.Status != Failed || result.Value != "" {
				t.Fatalf("invalid path/value accepted: %+v", result)
			}
		})
	}
}

func TestResponseConditionsAndMultipleRules(t *testing.T) {
	rules := []Rule{{Path: "$.token", Variable: "token"}, {Path: "$.missing", Variable: "other"}, {Path: "bad path", Variable: "ignored", Disabled: true}}
	results := Evaluate(201, []byte(`{"token":"secret"}`), true, rules)
	if results[0].Status != Success || results[1].Status != Failed || results[2].Status != Disabled {
		t.Fatalf("results = %+v", results)
	}
	for _, tt := range []struct {
		status   int
		body     string
		complete bool
		want     string
	}{
		{200, "not JSON", true, Failed}, {200, `{"token":"s"} trailing`, true, Failed}, {401, `{"token":"s"}`, true, Skipped}, {500, `{"token":"s"}`, true, Skipped}, {200, `{"token":"s"}`, false, Skipped},
	} {
		result := Evaluate(tt.status, []byte(tt.body), tt.complete, rules)[0]
		if result.Status != tt.want || result.Value != "" {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
	if results := Evaluate(200, nil, true, nil); results != nil {
		t.Fatal("requests without rules should do no work")
	}
}
