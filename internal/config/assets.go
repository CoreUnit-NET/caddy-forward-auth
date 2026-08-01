package config

import (
	"fmt"
	"os"
	"strconv"
)

func EnvIsString(envVar string, existCallback func(value string)) error {
	value := os.Getenv(envVar)
	if len(value) == 0 {
		return nil
	}

	existCallback(value)
	return nil
}

func EnvIsInt(envVar string, existCallback func(value int)) error {
	value := os.Getenv(envVar)
	if len(value) == 0 {
		return nil
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid %s value %q: want integer", envVar, value)
	}

	existCallback(intValue)
	return nil
}

func EnvIsBool(envVar string, existCallback func(value bool)) error {
	value := os.Getenv(envVar)
	if len(value) == 0 {
		return nil
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("invalid %s value %q: want boolean", envVar, value)
	}

	existCallback(boolValue)
	return nil
}
