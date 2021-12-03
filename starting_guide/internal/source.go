package internal

import (
	"embed"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
)

//go:embed data
var data embed.FS

type floatIterator func() float64

func getFloatIterator(input []float64) floatIterator {
	index := -1
	return func() float64 {
		index += 1
		if index >= len(input) {
			index = 0
		}

		return input[index]
	}
}

type boolIterator func() bool

func getBoolIterator(input []bool) boolIterator {
	index := -1
	return func() bool {
		index += 1
		if index >= len(input) {
			index = 0
		}

		return input[index]
	}
}

type DataSource struct {
	temperature floatIterator
	humidity    floatIterator
	light       boolIterator
}

func NewDataSource() (*DataSource, error) {
	f, err := data.Open("data/data.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = ' '
	temperature := make([]float64, 0, 3373)
	humidity := make([]float64, 0, 3373)
	light := make([]bool, 0, 3373)
	for {
		row, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return nil, err
			}
		}

		if len(row) != 3 {
			continue
		}

		t, err := strconv.ParseFloat(row[0], 64)
		if err == nil {
			temperature = append(temperature, t)
		}

		h, err := strconv.ParseFloat(row[1], 64)
		if err == nil {
			humidity = append(humidity, h)
		}

		if row[2] == "1" {
			light = append(light, true)
		} else {
			light = append(light, false)
		}
	}

	return &DataSource{
		temperature: getFloatIterator(temperature),
		humidity: getFloatIterator(humidity),
		light: getBoolIterator(light),
	}, nil
}

func (source *DataSource) GetTemperatureValue() float64 {
	return source.temperature()
}

func (source *DataSource) GetHumidityValue() float64 {
	return source.humidity()
}

func (source *DataSource) GetLightValue() bool {
	return source.light()
}
