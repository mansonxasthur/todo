package todo

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type metrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
}

func newMetrics() (metrics, error) {
	meter := otel.Meter("todo")

	created, err := meter.Int64Counter(
		"todo.created",
		metric.WithDescription("Number of todos created"),
		metric.WithUnit("{todo}"),
	)

	if err != nil {
		return metrics{}, err
	}

	completed, err := meter.Int64Counter(
		"todo.completed",
		metric.WithDescription("Number of todos marked done"),
		metric.WithUnit("{todo}"),
	)

	if err != nil {
		return metrics{}, err
	}

	return metrics{created: created, completed: completed}, nil
}
