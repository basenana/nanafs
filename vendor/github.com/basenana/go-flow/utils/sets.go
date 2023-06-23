package utils

type StringSet map[string]struct{}

func (s StringSet) Insert(strList ...string) {
	for _, str := range strList {
		s[str] = struct{}{}
	}
}
func (s StringSet) Has(str string) bool {
	_, ok := s[str]
	return ok
}

func (s StringSet) Del(str string) {
	if _, ok := s[str]; ok {
		delete(s, str)
	}
}

func (s StringSet) List() (result []string) {
	for k := range s {
		result = append(result, k)
	}
	return
}

func (s StringSet) Len() int {
	return len(s)
}

func NewStringSet(strList ...string) StringSet {
	ss := map[string]struct{}{}
	for _, str := range strList {
		ss[str] = struct{}{}
	}
	return ss
}
