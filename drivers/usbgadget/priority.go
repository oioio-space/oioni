// usbgadget/priority.go
package usbgadget

import (
	"github.com/oioio-space/oioni/drivers/usbgadget/functions"
	"sort"
)

var typePriority = map[string]int{
	"rndis":        0,
	"ecm":          1,
	"ncm":          2,
	"hid":          3,
	"mass_storage": 4,
	"acm":          5,
	"midi":         6,
}

func sortFunctions(funcs []functions.Function) []functions.Function {
	sorted := make([]functions.Function, len(funcs))
	copy(sorted, funcs)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, ok1 := typePriority[sorted[i].TypeName()]
		pj, ok2 := typePriority[sorted[j].TypeName()]
		if !ok1 {
			pi = 99
		}
		if !ok2 {
			pj = 99
		}
		return pi < pj
	})
	return sorted
}
