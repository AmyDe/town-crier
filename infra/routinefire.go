package main

import (
	"github.com/pulumi/pulumi-azure-native-sdk/logic/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/monitor/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const routineFireTriggerName = "manual"

const essentialsPath = "triggerBody()?['data']?['essentials']?"

// routineFireText carries only the alert essentials, which are all values we
// define on the alert rules, so the routine never receives log content.
const routineFireText = "Azure Monitor alert fired.\n" +
	"Rule: @{" + essentialsPath + "['alertRule']}\n" +
	"Severity: @{" + essentialsPath + "['severity']}\n" +
	"Signal: @{" + essentialsPath + "['signalType']}\n" +
	"Fired: @{" + essentialsPath + "['firedDateTime']}\n" +
	"Description: @{" + essentialsPath + "['description']}\n" +
	"Targets: @{string(" + essentialsPath + "['alertTargetIDs'])}"

func routineFireWorkflowDefinition() map[string]any {
	return map[string]any{
		"$schema":        "https://schema.management.azure.com/providers/Microsoft.Logic/schemas/2016-06-01/workflowdefinition.json#",
		"contentVersion": "1.0.0.0",
		"parameters": map[string]any{
			"routineFireUrl":   map[string]any{"type": "String"},
			"routineFireToken": map[string]any{"type": "SecureString"},
		},
		"triggers": map[string]any{
			routineFireTriggerName: map[string]any{
				"type":   "Request",
				"kind":   "Http",
				"inputs": map[string]any{"schema": map[string]any{}},
			},
		},
		"actions": map[string]any{
			"If_fired": map[string]any{
				"type":     "If",
				"runAfter": map[string]any{},
				"expression": map[string]any{
					"and": []any{
						map[string]any{"equals": []any{"@" + essentialsPath + "['monitorCondition']", "Fired"}},
					},
				},
				"actions": map[string]any{
					"Fire_routine": map[string]any{
						"type": "Http",
						"inputs": map[string]any{
							"method": "POST",
							"uri":    "@parameters('routineFireUrl')",
							"headers": map[string]any{
								"Authorization":     "Bearer @{parameters('routineFireToken')}",
								"anthropic-beta":    "experimental-cc-routine-2026-04-01",
								"anthropic-version": "2023-06-01",
								"Content-Type":      "application/json",
							},
							"body": map[string]any{"text": routineFireText},
						},
						// Without secureData the bearer token is readable in every run's history.
						"runtimeConfiguration": map[string]any{
							"secureData": map[string]any{"properties": []any{"inputs"}},
						},
					},
				},
			},
		},
		"outputs": map[string]any{},
	}
}

func createRoutineFireLogicApp(ctx *pulumi.Context, resourceGroup *resources.ResourceGroup, tags pulumi.StringMap, fireURL string, fireToken pulumi.StringOutput) (monitor.LogicAppReceiverArray, error) {
	workflow, err := logic.NewWorkflow(ctx, "logic-town-crier-routine-fire", &logic.WorkflowArgs{
		WorkflowName:      pulumi.String("logic-town-crier-routine-fire"),
		ResourceGroupName: resourceGroup.Name,
		Location:          pulumi.String("uksouth"),
		State:             pulumi.String("Enabled"),
		Definition:        pulumi.Any(routineFireWorkflowDefinition()),
		Parameters: logic.WorkflowParameterMap{
			"routineFireUrl":   logic.WorkflowParameterArgs{Type: pulumi.String("String"), Value: pulumi.String(fireURL)},
			"routineFireToken": logic.WorkflowParameterArgs{Type: pulumi.String("SecureString"), Value: fireToken},
		},
		Tags: tags,
	})
	if err != nil {
		return nil, err
	}

	callback := logic.ListWorkflowTriggerCallbackUrlOutput(ctx, logic.ListWorkflowTriggerCallbackUrlOutputArgs{
		ResourceGroupName: resourceGroup.Name,
		WorkflowName:      workflow.Name,
		TriggerName:       pulumi.String(routineFireTriggerName),
	})

	return monitor.LogicAppReceiverArray{
		monitor.LogicAppReceiverArgs{
			Name:                 pulumi.String("claude-routine"),
			ResourceId:           workflow.ID(),
			CallbackUrl:          pulumi.ToSecret(callback.Value()).(pulumi.StringOutput),
			UseCommonAlertSchema: pulumi.Bool(true),
		},
	}, nil
}
