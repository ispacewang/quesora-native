package main

// One inventory pass supplies both the preview and every quantity label.
type examFormStatus struct {
	Available         [5]int
	Actual, Requested int
	Shortage          bool
	Problem           string
	Valid             bool
}

func (a *app) examSetupStatus() examFormStatus {
	s := examFormStatus{}
	for _, q := range a.questions {
		if q.Bank == a.examForm.Bank {
			for i, kind := range examKinds {
				if q.Kind == kind {
					s.Available[i]++
					break
				}
			}
		}
	}
	if a.examForm.Bank == "" {
		return s
	}
	if err := validateExamSettings(a.examForm); err != nil {
		s.Problem = err.Error()
		return s
	}
	for i, v := range a.examForm.Counts {
		requested := int(v)
		s.Requested += requested
		s.Actual += min(requested, s.Available[i])
		s.Shortage = s.Shortage || requested > s.Available[i]
	}
	if s.Requested == 0 {
		s.Problem = "请至少设置一种题型的数量。"
	} else if s.Actual == 0 {
		s.Problem = "所选题型暂无题目，请调整题量或更换题库。"
	} else {
		s.Valid = true
	}
	return s
}
func (a *app) examFormEdited() { a.examMessage = "" }
