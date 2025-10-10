package cel

import (
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/pkg/errors"
	exprv1 "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

var defaultCELAttributes = []cel.EnvOption{
	// Current timestamp function.
	cel.Function("now",
		cel.Overload("now",
			[]*cel.Type{},
			cel.IntType,
			cel.FunctionBinding(func(_ ...ref.Val) ref.Val {
				return types.Int(time.Now().Unix())
			}),
		),
	),
}

func init() {
	for name, col := range identify2Columns {
		defaultCELAttributes = append(defaultCELAttributes, cel.Variable(name, col.valtype))
	}
}

// Parse parses the filter string and returns the parsed expression.
// The filter string should be a CEL expression.
func Parse(filter string, opts ...cel.EnvOption) (expr *exprv1.ParsedExpr, err error) {
	opts = append(opts, defaultCELAttributes...)
	e, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CEL environment")
	}
	ast, issues := e.Compile(filter)
	if issues != nil {
		return nil, errors.Errorf("failed to compile filter: %v", issues)
	}
	return cel.AstToParsedExpr(ast)
}
