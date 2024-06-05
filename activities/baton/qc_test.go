package batonactivities

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/baton"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *UnitTestSuite) TestQC() {
	// This test currently takes about 6 minutes and can only run locally in Moss.
	s.T().Skip("Too long")
	s.env.RegisterActivity(QC)
	s.env.ExecuteActivity(QC, QCParams{
		Path: paths.Path{
			Path:  "Production/masters/2024/3/22/a1d31721-b760-497d-9caa-a8e0a0c61615/PC24_TEMA_SKAPELSEN_KLICK_MAS.mov",
			Drive: paths.IsilonDrive,
		},
		Plan: baton.TestPlanMOV,
	})
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
