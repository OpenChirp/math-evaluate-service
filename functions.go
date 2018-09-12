package main

import (
	"fmt"
	"math"

	"github.com/Knetic/govaluate"
)

var evalFunctions = map[string]govaluate.ExpressionFunction{
	"strlen": func(args ...interface{}) (interface{}, error) {
		length := len(args[0].(string))
		return (float64)(length), nil
	},
	// log(100) = 2
	// log(4, 2) = 2
	"log": func(args ...interface{}) (interface{}, error) {
		if len(args) > 2 {
			return nil, fmt.Errorf("Too many arguments to log")
		}
		if len(args) < 1 {
			return nil, fmt.Errorf("Too few arguments to log")
		}

		var argsFloat64 [2]float64

		for i := range args {
			switch args[i].(type) {
			case float64:
				argsFloat64[i] = args[i].(float64)
			case bool:
				if args[i].(bool) {
					argsFloat64[i] = 1
				} else {
					argsFloat64[i] = 0
				}
			case string:
				return nil, fmt.Errorf("Argument %d is string to log", i)
			}
		}

		val := math.Log10(argsFloat64[0])
		if len(args) > 1 {
			val /= math.Log10(argsFloat64[1])
		}

		return val, nil
	},
}
