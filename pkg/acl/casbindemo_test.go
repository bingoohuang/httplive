package acl_test

import (
	"fmt"
	"github.com/bingoohuang/httplive/pkg/acl"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

func ExampleCasbindemo1() {
	// Initialize the model from a string.
	m, err := model.NewModelFromString(`
[request_definition]
r = user, db, tables, op

[policy_definition]
p = user, db, tables, op

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) 

[matchers]
m = r.user == "root" || g(r.user, p.user) && match(r.db, p.db) && match(r.tables, p.tables) && match(r.op, p.op) 
`)
	if err != nil {
		panic(err)
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		panic(err)
	}

	if err := acl.ResetPolicyString(m, `
p,bingoo,*,*,*
p,dingoo,db1/db2,*,read
g,b1,bingoo
`); err != nil {
		panic(err)
	}

	e.AddFunction("match", func(args ...interface{}) (interface{}, error) {
		return acl.WildcardMatch(args[0].(string), args[1].(string)), nil
	})

	fmt.Println(e.Enforce("bingoo", "db2", "table1", "write"))
	fmt.Println(e.Enforce("dingoo", "db1", "table1", "read"))
	fmt.Println(e.Enforce("dingoo", "db1", "table1", "write"))
	fmt.Println(e.Enforce("dingoo", "db2", "table1", "read"))
	fmt.Println(e.Enforce("dingoo", "db2", "table1", "write"))

	if err := acl.ResetPolicyString(m, `
p,bingoo,*,*,*
p,dingoo,db1/db2,*,read/write
`); err != nil {
		panic(err)
	}
	fmt.Println(e.Enforce("dingoo", "db2", "table1", "write"))
	fmt.Println(e.Enforce("root", "xx", "xx", "xx"))

	// Output:
	// true <nil>
	// true <nil>
	// false <nil>
	// true <nil>
	// false <nil>
	// true <nil>
	// true <nil>
}

func ExampleCasbindemo2() {
	m, err := model.NewModelFromString(m1)
	if err != nil {
		panic(err)
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		panic(err)
	}

	if err := acl.ResetPolicyString(m, p1); err != nil {
		panic(err)
	}

	e.AddFunction("timeAllow", func(args ...interface{}) (interface{}, error) {
		return acl.TimeAllow(args[0].(string), args[1].(string)), nil
	})

	for _, r := range acl.SplitLines(r1) {
		t, _ := acl.CsvTokens(r)
		fmt.Println(e.Enforce(t[0], t[1], t[2], t[3]))
	}

	// Output:
	// true <nil>
	// false <nil>
	// true <nil>
	// false <nil>
	// true <nil>
}

const p1 = `
p, alice, /alice_data/*, GET, 2020-12-16 17:37:55/2020-12-17 17:37:55
p, alice, /alice_data2/:id/using/:resId, GET, 2020-12-16 17:38:00
p, bob, /*, GET, -
`

const r1 = `
bob, /alice_data/hello, GET, 2020-12-16 17:37:00
alice, /alice_data/hello, GET, 2020-12-16 17:37:00
alice, /alice_data/hello, GET, 2020-12-17 17:37:00
alice, /alice_data/hello, POST, 2020-12-16 17:37:00
root, /alice_data/hello, POST, 2020-12-16 17:37:00
`

const m1 = `
[request_definition]
r = sub, obj, act, time

[policy_definition]
p = sub, obj, act, time

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == "root" || g(r.sub, p.sub) && r.sub == p.sub && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act) && timeAllow(r.time, p.time)
`
