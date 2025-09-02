package models

import "time"

type Params struct {
	Param     string   `json:"param" validate:"required"`
	Value     string   `json:"value,omitempty"`
	AnyValue  []string `json:"any_value,omitempty"`
	AllValues []string `json:"all_values,omitempty"`
}

type Rate struct {
	Limit  int64         `json:"limit" validate:"min=0"`
	Window time.Duration `json:"window" validate:"required"`
}

type Permission struct {
	Microservice     string   `json:"microservice" validate:"required"`
	Method           string   `json:"method" validate:"required"`
	Path             string   `json:"path" validate:"required"`
	Rates            []Rate   `json:"rates"`
	RequiredParams   []Params `json:"required_params"`
	RestrictedParams []Params `json:"restricted_params"`
}

type CompiledPermission struct {
	Microservice     string   `json:"microservice"`
	Method           string   `json:"method"`
	Path             string   `json:"path"`
	Rates            []Rate   `json:"rates"`
	RequiredParams   []Params `json:"required_params"`
	RestrictedParams []Params `json:"restricted_params"`
	InheritedFrom    []string `json:"inherited_from,omitempty"`
}
