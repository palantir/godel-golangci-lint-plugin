// Package analyzer contains tools for analyzing arangodb usage.
//
// Scope and limits of the analysis:
//   - Intra-procedural only: we do not follow calls across function boundaries.
//   - Flow/block sensitive within the current function: we scan statements that
//     occur before a call site in the nearest block and its ancestor blocks.
//   - Conservative by design: when options come from an unknown factory/helper
//     call, we assume AllowImplicit is set to prevent false positives.
//
// The analyzer focuses on github.com/arangodb/go-driver/v2.
package analyzer

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	allowImplicitFieldName   = "AllowImplicit"
	msgMissingAllowImplicit  = "missing AllowImplicit option"
	methodBeginTransaction   = "BeginTransaction"
	expectedBeginTxnArgs     = 3
	arangoDatabaseTypeSuffix = "github.com/arangodb/go-driver/v2/arangodb.Database"
	arangoPackageSuffix      = "github.com/arangodb/go-driver/v2/arangodb"
)

var errInvalidAnalysis = errors.New("invalid analysis")

// NewAnalyzer returns an arangolint analyzer.
func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "arangolint",
		Doc:      "opinionated best practices for arangodb client",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspctr, typeValid := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !typeValid {
		return nil, errInvalidAnalysis
	}

	// Visit only call expressions and get the traversal stack from the inspector.
	nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}
	inspctr.WithStack(nodeFilter, func(node ast.Node, push bool, stack []ast.Node) (proceed bool) {
		if !push {
			return true
		}

		// node is guaranteed to be *ast.CallExpr due to the filter above.
		call := node.(*ast.CallExpr) //nolint:forcetypeassert
		handleBeginTransactionCall(call, pass, stack)

		return true
	})

	return nil, nil //nolint:nilnil
}

// handleBeginTransactionCall validates BeginTransaction(...) call sites.
// Analysis is intra-procedural and flow/block-sensitive: it scans statements
// that appear before the call within the nearest and ancestor blocks.
// For third-argument values produced by unknown factory/helper calls, the
// analyzer remains conservative (assumes AllowImplicit) to avoid
// false positives that could annoy users.
func handleBeginTransactionCall(call *ast.CallExpr, pass *analysis.Pass, stack []ast.Node) {
	if !isBeginTransaction(call, pass) {
		return
	}

	diag := analysis.Diagnostic{
		Pos:     call.Pos(),
		Message: msgMissingAllowImplicit,
	}

	// Normalize the 3rd argument by unwrapping parentheses
	arg := unwrapParens(call.Args[2])

	switch optsExpr := arg.(type) {
	case *ast.Ident:
		if isNilIdent(optsExpr) {
			pass.Report(diag)

			return
		}

		if hasAllowImplicitForIdent(optsExpr, pass, stack, call.Pos()) {
			return
		}

		pass.Report(diag)
	case *ast.UnaryExpr:
		// &CompositeLit or &ident
		if has, ok := compositeAllowsImplicit(optsExpr); ok {
			if !has {
				pass.Report(diag)
			}

			return
		}

		// not a composite literal, try &ident
		if id, ok := optsExpr.X.(*ast.Ident); ok {
			if hasAllowImplicitForIdent(id, pass, stack, call.Pos()) {
				return
			}

			pass.Report(diag)
		}
	case *ast.SelectorExpr:
		// s.opts (or nested) passed as options
		if hasAllowImplicitForSelector(optsExpr, pass, stack, call.Pos()) {
			return
		}

		pass.Report(diag)
	case *ast.CallExpr:
		// Typed conversion like (*arangodb.BeginTransactionOptions)(nil)
		if isTypeConversionToTxnOptionsPtrNil(optsExpr, pass) {
			pass.Report(diag)

			return
		}
		// For other calls (factory/helpers), we stay conservative to avoid false positives.
	}
}

func unwrapParens(arg ast.Expr) ast.Expr {
	for {
		switch pe := arg.(type) {
		case *ast.ParenExpr:
			arg = pe.X
		default:
			return arg
		}
	}
}

// isNilIdent reports whether e is an identifier named "nil".
func isNilIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)

	return ok && id.Name == "nil"
}

// isAllowImplicitSelector reports whether s selects the AllowImplicit field.
func isAllowImplicitSelector(s *ast.SelectorExpr) bool {
	return s != nil && s.Sel != nil && s.Sel.Name == allowImplicitFieldName
}

// isBeginTransaction reports whether call is a call to arangodb.Database.BeginTransaction.
// It prefers selection-based detection via TypesInfo.Selections to support wrappers or
// types that embed arangodb.Database. If selection info is unavailable, it falls back
// to checking the receiver type's string suffix for .../arangodb.Database to handle
// aliases or named types that preserve the type name.
func isBeginTransaction(call *ast.CallExpr, pass *analysis.Pass) bool {
	selExpr, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	if selExpr.Sel == nil || selExpr.Sel.Name != methodBeginTransaction {
		return false
	}

	// Validate expected args count with extracted constant for clarity
	if len(call.Args) != expectedBeginTxnArgs {
		return false
	}

	xType := pass.TypesInfo.TypeOf(selExpr.X)
	if xType == nil {
		return false
	}

	// Try to find the arangodb package in the current package imports and get the Database type.
	var dbType types.Type

	for _, imp := range pass.Pkg.Imports() {
		if strings.HasSuffix(imp.Path(), arangoPackageSuffix) {
			if obj := imp.Scope().Lookup("Database"); obj != nil {
				if tn, ok := obj.(*types.TypeName); ok {
					dbType = tn.Type()
				}
			}

			break
		}
	}

	if dbType != nil {
		// If the receiver's type is assignable to arangodb.Database, it's a valid BeginTransaction call.
		if types.AssignableTo(xType, dbType) {
			return true
		}
	}

	// Last resort: direct receiver type match or alias that preserves the type name suffix
	return strings.HasSuffix(xType.String(), arangoDatabaseTypeSuffix)
}

// hasAllowImplicitForSelector checks if a selector expression (e.g., s.opts)
// has had its AllowImplicit field set prior to the call position within
// the nearest or any ancestor block. This is a conservative intra-procedural check.
func hasAllowImplicitForSelector(
	sel *ast.SelectorExpr,
	pass *analysis.Pass,
	stack []ast.Node,
	callPos token.Pos,
) bool {
	root := rootIdent(sel)
	if root == nil {
		return false
	}

	rootObj := pass.TypesInfo.ObjectOf(root)
	if rootObj == nil {
		return false
	}

	blocks := ancestorBlocks(stack)

	return scanPriorStatements(blocks, callPos, func(stmt ast.Stmt) bool {
		return setsAllowImplicitForObjectInAssign(stmt, rootObj, pass)
	})
}

// setsAllowImplicitForObjectInAssign reports true if the statement assigns to
// X.AllowImplicit and the root identifier of X matches the provided object.
func setsAllowImplicitForObjectInAssign(stmt ast.Stmt, obj types.Object, pass *analysis.Pass) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok {
		return false
	}

	for _, lhs := range assign.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		if !isAllowImplicitSelector(sel) {
			continue
		}

		r := rootIdent(sel.X)
		if r == nil {
			continue
		}

		if pass.TypesInfo.ObjectOf(r) == obj {
			return true
		}
	}

	return false
}

// hasAllowImplicitForIdent checks whether the given identifier (variable or pointer to options)
// has the AllowImplicit option explicitly set before the call position within the nearest or any ancestor block.
func hasAllowImplicitForIdent(
	id *ast.Ident,
	pass *analysis.Pass,
	stack []ast.Node,
	callPos token.Pos,
) bool {
	obj := pass.TypesInfo.ObjectOf(id)
	if obj == nil {
		return false
	}

	blocks := ancestorBlocks(stack)
	// Walk from the nearest block outward and scan statements before the call position
	if scanPriorStatements(blocks, callPos, func(stmt ast.Stmt) bool {
		return stmtSetsAllowImplicitForObj(stmt, obj, pass)
	}) {
		return true
	}

	// If not found in local/ancestor blocks, also check for package-level (global)
	// variable declarations that initialize AllowImplicit.
	if hasAllowImplicitForPackageVar(pass, obj) {
		return true
	}

	return false
}

// ancestorBlocks returns the list of enclosing blocks for the current node, from
// nearest to outermost. This supports intra-procedural, flow-sensitive scans of
// statements that occur before the call site.
func ancestorBlocks(stack []ast.Node) []*ast.BlockStmt {
	var blks []*ast.BlockStmt
	for i := len(stack) - 1; i >= 0; i-- {
		if blk, ok := stack[i].(*ast.BlockStmt); ok {
			blks = append(blks, blk)
		}
	}

	return blks
}

// scanPriorStatements iterates statements in the provided blocks in lexical order,
// visiting only statements that appear before the provided 'until' position. It stops
// early and returns true when visit returns true.
func scanPriorStatements(blocks []*ast.BlockStmt, until token.Pos, visit func(ast.Stmt) bool) bool {
	for _, blk := range blocks {
		for _, stmt := range blk.List {
			if stmt == nil {
				continue
			}

			if stmt.Pos() >= until {
				break
			}

			if visit(stmt) {
				return true
			}
		}
	}

	return false
}

func initHasAllowImplicitForObj(
	assign *ast.AssignStmt,
	obj types.Object,
	pass *analysis.Pass,
) bool {
	// find the RHS corresponding to our obj
	for lhsIndex, lhs := range assign.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}

		if pass.TypesInfo.ObjectOf(id) != obj {
			continue
		}

		var rhsValue ast.Expr

		switch {
		case len(assign.Rhs) == len(assign.Lhs):
			rhsValue = assign.Rhs[lhsIndex]
		case len(assign.Rhs) == 1:
			rhsValue = assign.Rhs[0]
		default:
			continue
		}

		// Check for AllowImplicit in either &CompositeLit or CompositeLit via helper
		if has, ok := compositeAllowsImplicit(rhsValue); ok {
			return has
		}
	}

	return false
}

func declInitHasAllowImplicitForObj(stmt ast.Stmt, obj types.Object, pass *analysis.Pass) bool {
	declStmt, isDeclStmt := stmt.(*ast.DeclStmt)
	if !isDeclStmt {
		return false
	}

	genDecl, isGenDecl := declStmt.Decl.(*ast.GenDecl)
	if !isGenDecl || genDecl.Tok != token.VAR {
		return false
	}

	for _, spec := range genDecl.Specs {
		valueSpec, isValueSpec := spec.(*ast.ValueSpec)
		if !isValueSpec {
			continue
		}

		if valueSpecHasAllowImplicitForObj(valueSpec, obj, pass) {
			return true
		}
	}

	return false
}

func valueSpecHasAllowImplicitForObj(
	valueSpec *ast.ValueSpec,
	obj types.Object,
	pass *analysis.Pass,
) bool {
	// find the index corresponding to our obj
	targetIndex := -1

	for i, name := range valueSpec.Names {
		if pass.TypesInfo.ObjectOf(name) == obj {
			targetIndex = i

			break
		}
	}

	if targetIndex == -1 {
		return false
	}

	// pick the value expression for this name
	var rhsValue ast.Expr

	switch {
	case targetIndex < len(valueSpec.Values):
		rhsValue = valueSpec.Values[targetIndex]
	case len(valueSpec.Values) == 1:
		rhsValue = valueSpec.Values[0]
	default:
		return false
	}

	// Check for AllowImplicit in either &CompositeLit or CompositeLit via helper
	if has, ok := compositeAllowsImplicit(rhsValue); ok {
		return has
	}

	return false
}

func stmtSetsAllowImplicitForObj(stmt ast.Stmt, obj types.Object, pass *analysis.Pass) bool {
	// Direct assignment like opts.AllowImplicit = true
	if setsAllowImplicitForObjectInAssign(stmt, obj, pass) {
		return true
	}

	// Variable initialization via assignment (short var or regular assignment)
	if assign, ok := stmt.(*ast.AssignStmt); ok {
		if initHasAllowImplicitForObj(assign, obj, pass) {
			return true
		}
	}

	// Variable declaration with initialization
	if declInitHasAllowImplicitForObj(stmt, obj, pass) {
		return true
	}

	// Control-flow constructs that may contain relevant prior mutations/initializations
	switch stmtNode := stmt.(type) {
	case *ast.IfStmt:
		if handleIfAllowImplicit(stmtNode, obj, pass) {
			return true
		}
	case *ast.ForStmt:
		if handleForAllowImplicit(stmtNode, obj, pass) {
			return true
		}
	case *ast.SwitchStmt:
		if handleSwitchAllowImplicit(stmtNode, obj, pass) {
			return true
		}
	}

	return false
}

func rootIdent(expr ast.Expr) *ast.Ident {
	for {
		switch typedExpr := expr.(type) {
		case *ast.Ident:
			return typedExpr
		case *ast.ParenExpr:
			expr = typedExpr.X
		case *ast.StarExpr:
			expr = typedExpr.X
		case *ast.SelectorExpr:
			// walk down the selector chain until we hit the root identifier
			expr = typedExpr.X
		default:
			return nil
		}
	}
}

// isTypeConversionToTxnOptionsPtrNil reports whether call is a type conversion to a
// pointer type with a single nil argument, e.g. (*arangodb.BeginTransactionOptions)(nil).
// This recognizes explicit nil options passed via a cast.
func isTypeConversionToTxnOptionsPtrNil(call *ast.CallExpr, pass *analysis.Pass) bool {
	// single arg must be a nil identifier
	if len(call.Args) != 1 {
		return false
	}

	if !isNilIdent(call.Args[0]) {
		return false
	}
	// Check the target type is a pointer type
	if t := pass.TypesInfo.TypeOf(call.Fun); t != nil {
		if _, ok := t.(*types.Pointer); ok {
			return true
		}
	}
	// Fallback to a syntactic check
	fun := call.Fun
	for {
		if p, ok := fun.(*ast.ParenExpr); ok {
			fun = p.X

			continue
		}

		break
	}

	_, ok := fun.(*ast.StarExpr)

	return ok
}

// hasAllowImplicitForPackageVar scans all files for top-level var declarations
// of the given object and returns true if its initialization sets AllowImplicit.
func hasAllowImplicitForPackageVar(pass *analysis.Pass, obj types.Object) bool {
	// Only variables can be relevant here, but the object identity check below
	// will safely no-op for others.
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}

			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				if valueSpecHasAllowImplicitForObj(valueSpec, obj, pass) {
					return true
				}
			}
		}
	}

	return false
}

// compositeAllowsImplicit reports whether expr is a composite literal (or address-of one)
// that contains a KeyValueExpr with key ident named allowImplicitFieldName ("AllowImplicit").
// It returns (has, ok) where ok indicates the expression was a recognized composite literal shape.
func compositeAllowsImplicit(expr ast.Expr) (bool, bool) {
	expr = unwrapParens(expr)

	// handle address-of &CompositeLit
	if ue, ok := expr.(*ast.UnaryExpr); ok {
		expr = unwrapParens(ue.X)
	}

	// handle CompositeLit
	if cl, ok := expr.(*ast.CompositeLit); ok {
		for _, elt := range cl.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == allowImplicitFieldName {
					return true, true
				}
			}
		}

		return false, true
	}

	return false, false
}

// handleIfAllowImplicit scans the if statement's body and else branches for assignments or initializations
// that set AllowImplicit for the given object. Behavior mirrors the inline logic previously in
// stmtSetsAllowImplicitForObj; extracted for readability only.
func handleIfAllowImplicit(stmtNode *ast.IfStmt, obj types.Object, pass *analysis.Pass) bool {
	// Recurse into body statements
	for _, st := range stmtNode.Body.List {
		if stmtSetsAllowImplicitForObj(st, obj, pass) {
			return true
		}
	}
	// Else can be another IfStmt (else-if) or a BlockStmt
	switch elseNode := stmtNode.Else.(type) {
	case *ast.BlockStmt:
		for _, st := range elseNode.List {
			if stmtSetsAllowImplicitForObj(st, obj, pass) {
				return true
			}
		}
	case *ast.IfStmt:
		if stmtSetsAllowImplicitForObj(elseNode, obj, pass) {
			return true
		}
	}

	return false
}

// handleForAllowImplicit scans a for statement's init and body for relevant initializations/assignments.
func handleForAllowImplicit(stmtNode *ast.ForStmt, obj types.Object, pass *analysis.Pass) bool {
	// e.g., for i := 0; i < n; i++ { opts.AllowImplicit = true }
	if assign, ok := stmtNode.Init.(*ast.AssignStmt); ok {
		if initHasAllowImplicitForObj(assign, obj, pass) {
			return true
		}
	}

	for _, st := range stmtNode.Body.List {
		if stmtSetsAllowImplicitForObj(st, obj, pass) {
			return true
		}
	}

	return false
}

// handleSwitchAllowImplicit scans a switch statement's init and case bodies.
func handleSwitchAllowImplicit(
	stmtNode *ast.SwitchStmt,
	obj types.Object,
	pass *analysis.Pass,
) bool {
	if assign, ok := stmtNode.Init.(*ast.AssignStmt); ok {
		if initHasAllowImplicitForObj(assign, obj, pass) {
			return true
		}
	}

	for _, cc := range stmtNode.Body.List {
		if clause, ok := cc.(*ast.CaseClause); ok {
			for _, st := range clause.Body {
				if stmtSetsAllowImplicitForObj(st, obj, pass) {
					return true
				}
			}
		}
	}

	return false
}
