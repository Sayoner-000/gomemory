package console

import (
	"encoding/json"
	"os"
	"testing"
)

func TestThemeResolutionSharedCases(t *testing.T) {
	data,err:=os.ReadFile("../../../assets/theme-resolution-cases.json")
	if err!=nil {t.Fatal(err)}
	var cases []struct{Name string; Env map[string]string; Expected string}
	if err:=json.Unmarshal(data,&cases);err!=nil {t.Fatal(err)}
	for _,tc:=range cases {
		t.Run(tc.Name,func(t *testing.T){
			p:=Theme(func(k string)string{return tc.Env[k]})
			if p.Name!=tc.Expected {t.Fatalf("%v: tema %q, esperado %q",tc.Env,p.Name,tc.Expected)}
		})
	}
}
