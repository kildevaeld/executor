package socket

import "github.com/kildevaeld/executor"

type ServiceList []executor.ServiceDesc

func (self ServiceList) Get(name string) *executor.ServiceDesc {

	for _, service := range self {
		if service.Name == name {
			return &service
		}
	}
	return nil
}

func (self ServiceList) Has(name string) bool {
	return self.Get(name) != nil
}

func (self *ServiceList) Append(services ...executor.ServiceDesc) {
	for _, service := range services {
		if self.Has(service.Name) {

			for i, s := range *self {
				if s.Name == service.Name {
					*self = append((*self)[:i], (*self)[i+1:]...)
					break
				}
			}

		}

		*self = append(*self, service)
	}
}
