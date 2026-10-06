package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoutineFireWorkflowDefinition_PostsFiredAlertsToRoutine(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(routineFireWorkflowDefinition())
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	var def struct {
		Parameters map[string]struct {
			Type string `json:"type"`
		} `json:"parameters"`
		Triggers map[string]struct {
			Type string `json:"type"`
			Kind string `json:"kind"`
		} `json:"triggers"`
		Actions map[string]struct {
			Type       string `json:"type"`
			Expression struct {
				And []struct {
					Equals []string `json:"equals"`
				} `json:"and"`
			} `json:"expression"`
			Actions map[string]struct {
				Type   string `json:"type"`
				Inputs struct {
					Method  string            `json:"method"`
					URI     string            `json:"uri"`
					Headers map[string]string `json:"headers"`
					Body    struct {
						Text string `json:"text"`
					} `json:"body"`
				} `json:"inputs"`
				RuntimeConfiguration struct {
					SecureData struct {
						Properties []string `json:"properties"`
					} `json:"secureData"`
				} `json:"runtimeConfiguration"`
			} `json:"actions"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatalf("unmarshal definition: %v", err)
	}

	if got := def.Parameters["routineFireToken"].Type; got != "SecureString" {
		t.Errorf("routineFireToken parameter type = %q, want SecureString", got)
	}
	trigger, ok := def.Triggers[routineFireTriggerName]
	if !ok || trigger.Type != "Request" || trigger.Kind != "Http" {
		t.Errorf("trigger %q = %+v, want an HTTP Request trigger", routineFireTriggerName, trigger)
	}

	cond, ok := def.Actions["If_fired"]
	if !ok || cond.Type != "If" || len(cond.Expression.And) != 1 {
		t.Fatalf("If_fired action = %+v, want a single-clause If", cond)
	}
	if got := cond.Expression.And[0].Equals; len(got) != 2 ||
		got[0] != "@triggerBody()?['data']?['essentials']?['monitorCondition']" || got[1] != "Fired" {
		t.Errorf("If_fired condition = %v, want monitorCondition == Fired", got)
	}

	fire, ok := cond.Actions["Fire_routine"]
	if !ok || fire.Type != "Http" {
		t.Fatalf("Fire_routine action = %+v, want an Http action", fire)
	}
	if fire.Inputs.Method != "POST" || fire.Inputs.URI != "@parameters('routineFireUrl')" {
		t.Errorf("Fire_routine request = %s %s", fire.Inputs.Method, fire.Inputs.URI)
	}
	wantHeaders := map[string]string{
		"Authorization":     "Bearer @{parameters('routineFireToken')}",
		"anthropic-beta":    "experimental-cc-routine-2026-04-01",
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
	}
	for k, want := range wantHeaders {
		if got := fire.Inputs.Headers[k]; got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
	if got := fire.RuntimeConfiguration.SecureData.Properties; len(got) != 1 || got[0] != "inputs" {
		t.Errorf("secureData properties = %v, want [inputs] so the token stays out of run history", got)
	}

	text := fire.Inputs.Body.Text
	if !strings.Contains(text, "['essentials']?['alertRule']") {
		t.Errorf("fire text does not name the alert rule: %q", text)
	}
	if strings.Contains(text, "alertContext") {
		t.Errorf("fire text must carry only alert essentials, got %q", text)
	}
}
