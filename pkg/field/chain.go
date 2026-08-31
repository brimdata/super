package field

type ChainElem struct {
	ID      string
	Noneish bool
}

type Chain []ChainElem

func NewChain(ids ...string) Chain {
	path := make([]ChainElem, 0, len(ids))
	for _, id := range ids {
		path = append(path, ChainElem{ID: id})
	}
	return path
}

func (c Chain) Append(id string, noneish bool) Chain {
	return append(c, ChainElem{id, noneish})
}

func (c Chain) Path() Path {
	path := make([]string, 0, len(c))
	for _, elem := range c {
		path = append(path, elem.ID)
	}
	return path
}

func (c Chain) String() string {
	return c.Path().String()
}
