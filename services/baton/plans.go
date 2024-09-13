package baton

import "github.com/orsinium-labs/enum"

type TestPlan enum.Member[string]

var (
	TestPlanMOV   = TestPlan{Value: "ProRes Test"}
	TestPlanMXF   = TestPlan{Value: "BTV AVC Intra 100"}
	TestPlanBasic = TestPlan{Value: "BASIC Sanity Check"}
	TestPlans     = enum.New(TestPlanMOV, TestPlanMXF, TestPlanBasic)
)
