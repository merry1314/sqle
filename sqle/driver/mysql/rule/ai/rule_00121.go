package ai

import (
	rulepkg "github.com/actiontech/sqle/sqle/driver/mysql/rule"
	util "github.com/actiontech/sqle/sqle/driver/mysql/rule/ai/util"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/pingcap/parser/ast"

	"github.com/actiontech/sqle/sqle/driver/mysql/plocale"
)

const (
	SQLE00121 = "SQLE00121"
)

func init() {
	rh := rulepkg.SourceHandler{
		Rule: rulepkg.SourceRule{
			Name:       SQLE00121,
			Desc:       plocale.Rule00121Desc,
			Annotation: plocale.Rule00121Annotation,
			Category:   plocale.RuleTypeDMLConvention,
			CategoryTags: map[string][]string{
				plocale.RuleCategoryOperand.ID:              {plocale.RuleTagBusiness.ID},
				plocale.RuleCategorySQL.ID:                  {plocale.RuleTagDML.ID},
				plocale.RuleCategoryAuditPurpose.ID:         {plocale.RuleTagCorrection.ID},
				plocale.RuleCategoryAuditAccuracy.ID:        {plocale.RuleTagOffline.ID},
				plocale.RuleCategoryAuditPerformanceCost.ID: {},
			},
			Level:        driverV2.RuleLevelNotice,
			Params:       []*rulepkg.SourceParam{},
			Knowledge:    driverV2.RuleKnowledge{},
			AllowOffline: true,
			Version:      2,
		},
		Message: plocale.Rule00121Message,
		Func:    RuleSQLE00121,
	}
	sourceRuleHandlers = append(sourceRuleHandlers, &rh)
}

/*
==== Prompt start ====
In MySQL, you should check if the SQL violate the rule(SQLE00121): "For dml, It is recommended that you use ORDER BY in queries that limit the number of records".
You should follow the following logic:
1. For "SELECT..." The statement,
  1. Check if the limit keyword is present in the sentence, and if so, check further.
  2. Check if there is an order by clause in the sentence, if not, report a rule violation.
2. For INSERT... Statement to perform the same check as above on the SELECT clause in the INSERT statement.
3. For UNION... Statement, does the same check as above for each SELECT clause in the statement.
4. For UPDATE... Statement, the same checks as above are performed for the sub-queries in the statement.
5. For DELETE... Statement, the same checks as above are performed for the sub-queries in the statement.
==== Prompt end ====
*/

// ==== Rule code start ====
func RuleSQLE00121(input *rulepkg.RuleHandlerInput) error {
	switch stmt := input.Node.(type) {
	case *ast.SelectStmt, *ast.UnionStmt, *ast.InsertStmt, *ast.UpdateStmt, *ast.DeleteStmt:
		// "SELECT...", "UNION...", "INSERT...", "UPDATE...", "DELETE..."
		for _, selectStmt := range util.GetSelectStmt(stmt) {
			// "SELECT..."
			// Check if the limit keyword is present in the sentence
			if selectStmt.Limit != nil {
				// "LIMIT" is present in the SQL statement
				if selectStmt.OrderBy == nil || len(selectStmt.OrderBy.Items) == 0 {
					//"ORDER BY" is not present in the SQL statement
					rulepkg.AddResult(input.Res, input.Rule, SQLE00121)
					return nil
				}
			}
		}
	}
	return nil
}

// ==== Rule code end ====
