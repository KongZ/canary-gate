/*
Copyright 2025 The canary-gate authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

var (
	eventGvr = corev1.SchemeGroupVersion.WithResource("events")
)

// DynamicEventSink implements the record.EventSink interface using a dynamic client.
type DynamicEventSink struct {
	Client dynamic.Interface
}

// Update implements record.EventSink.
func (s *DynamicEventSink) Update(event *corev1.Event) (*corev1.Event, error) {
	panic("unimplemented")
}

// Create sends an event to the API server using the dynamic client.
func (s *DynamicEventSink) Create(event *corev1.Event) (*corev1.Event, error) {
	// Convert the typed Event into an unstructured map.
	unstructuredEvent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event)
	if err != nil {
		return nil, fmt.Errorf("failed to convert event to unstructured: %w", err)
	}

	// Use the dynamic client to create the resource.
	createdUnstructured, err := s.Client.Resource(eventGvr).Namespace(event.Namespace).Create(context.Background(), &unstructured.Unstructured{Object: unstructuredEvent}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create event with dynamic client: %w", err)
	}

	// Convert the unstructured result back into a typed Event.
	var createdEvent corev1.Event
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(createdUnstructured.Object, &createdEvent); err != nil {
		return nil, fmt.Errorf("failed to convert created unstructured object to event: %w", err)
	}

	return &createdEvent, nil
}

// Patch is required by the EventSink interface. For this example, we won't implement it.
func (s *DynamicEventSink) Patch(event *corev1.Event, patch []byte) (*corev1.Event, error) {
	// A real implementation would use the dynamic client's Patch method.
	return event, nil
}
