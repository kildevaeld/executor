package cmd

import "github.com/kildevaeld/dict"

type Service struct {
}

func (self *Service) Create(name string, age int, out *dict.Map) error {
	*out = dict.Map{
		"Hello": "to you, " + name,
	}

	return nil
}

func (self *Service) Run(name string, age int, out *dict.Map) error {
	*out = dict.Map{
		"Hello": "to you, " + name,
	}

	return nil
}
