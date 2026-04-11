package venvs

type ViewModel struct {
	Width  int
	Height int
}

func NewViewModel() ViewModel {
	return ViewModel{}
}

func (m ViewModel) View() string {
	return "Venvs workspace ready."
}
