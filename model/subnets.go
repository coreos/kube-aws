package model

type Subnets []Subnet

func (s Subnets) ContainsBothPrivateAndPublic() bool {
	allPublic := true
	allPrivate := true
	for _, subnet := range s {
		allPublic = allPublic && subnet.Public()
		allPrivate = allPrivate && subnet.Private
	}
	return !allPublic && !allPrivate
}
