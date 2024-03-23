package content

import "fmt"

type ContentKind string

const (
	KindChallenge ContentKind = "challenge"
	KindTutorial  ContentKind = "tutorial"
	KindCourse    ContentKind = "course"
)

func (k *ContentKind) Set(v string) error {
	switch string(v) {
	case string(KindChallenge):
		*k = KindChallenge
	case string(KindTutorial):
		*k = KindTutorial
	case string(KindCourse):
		*k = KindCourse
	default:
		return fmt.Errorf("unknown content kind: %s", v)
	}

	return nil
}

func (k *ContentKind) String() string {
	return string(*k)
}

func (k *ContentKind) Plural() string {
	switch *k {
	case KindChallenge:
		return "challenges"
	case KindTutorial:
		return "tutorials"
	case KindCourse:
		return "courses"
	default:
		panic(fmt.Sprintf("unknown content kind: %s", k))
	}
}

func (k *ContentKind) Type() string {
	return "content-kind"
}

type Content interface {
	GetKind() ContentKind
	GetName() string
	GetPageURL() string
}
