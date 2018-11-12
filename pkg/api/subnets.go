package api

import "fmt"

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

func (ss Subnets) ImportFromNetworkStack() (Subnets, error) {
	result := make(Subnets, len(ss))
	// Import all the managed subnets from the main cluster i.e. don't create subnets inside the node pool cfn stack
	for i, s := range ss {
		if !s.HasIdentifier() {
			logicalName, err := s.LogicalNameOrErr()
			if err != nil {
				return result, err
			}
			stackOutputName := fmt.Sprintf(`{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-%s"}}`, logicalName)
			az := s.AvailabilityZone
			if s.Private {
				result[i] = NewPrivateSubnetFromFn(az, stackOutputName)
			} else {
				result[i] = NewPublicSubnetFromFn(az, stackOutputName)
			}
		} else {
			result[i] = s
		}
	}
	return result, nil
}

func (ss Subnets) ImportFromNetworkStackRetainingNames() (Subnets, error) {
	result, err := ss.ImportFromNetworkStack()
	if err != nil {
		return result, err
	}
	for i, s := range ss {
		result[i].Name = s.Name
	}
	return result, nil
}
