package domain

// QuestionKinds returns the supported types in examination order.
func QuestionKinds() [5]string {
	return [5]string{"单选题", "多选题", "判断题", "简答题", "填空题"}
}
