// Modified for Quesora: derive column bounds when BIFF ROW metadata is absent.
package xls

type rowInfo struct {
	Index    uint16
	Fcell    uint16
	Lcell    uint16
	Height   uint16
	Notused  uint16
	Notused2 uint16
	Flags    uint32
}

//Row the data of one row
type Row struct {
	wb   *WorkBook
	info *rowInfo
	cols map[uint16]contentHandler
}

//Col Get the Nth Col from the Row, if has not, return nil.
//Suggest use Has function to test it.
func (r *Row) Col(i int) string {
	serial := uint16(i)
	if ch, ok := r.cols[serial]; ok {
		strs := ch.String(r.wb)
		return strs[0]
	} else {
		for _, v := range r.cols {
			if v.FirstCol() <= serial && v.LastCol() >= serial {
				strs := v.String(r.wb)
				return strs[serial-v.FirstCol()]
			}
		}
	}
	return ""
}

//LastCol Get the number of Last Col of the Row.
func (r *Row) LastCol() int {
	last := int(r.info.Lcell)
	for _, col := range r.cols {
		if n := int(col.LastCol()) + 1; n > last {
			last = n
		}
	}
	return last
}

//FirstCol Get the number of First Col of the Row.
func (r *Row) FirstCol() int {
	return int(r.info.Fcell)
}
