package canal

import (
	"regexp"
)

type filterTable struct {
	include []string
	exclude []string
	table   []string
}

func FilterTable() *filterTable {

	return &filterTable{}
}

// [mysql\\..*]  is 'mysql' database all tables
func (f *filterTable) Include(table ...string) *filterTable {

	f.include = append(f.include, table...)
	return f

}
func (f *filterTable) Exclude(table ...string) *filterTable {

	f.exclude = append(f.exclude, table...)
	return f

}

func (f *filterTable) Match(table string) bool {

	var matchFlag bool
	if f.include == nil {
		matchFlag = true
	}

	for _, val := range f.include {
		reg, err := regexp.Compile(val)
		if err != nil {
			continue
		}

		if reg.MatchString(table) {
			matchFlag = true
			break
		}

	}

	for _, val := range f.exclude {
		reg, err := regexp.Compile(val)
		if err != nil {
			continue
		}
		if reg.MatchString(table) {
			matchFlag = false
			break
		}

	}
	return matchFlag
}
