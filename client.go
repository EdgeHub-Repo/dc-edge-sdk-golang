package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/amenzhinsky/iothub/iotdevice"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Client interface {
	Connect() error
	Disconnect(topic, payload string) error
	IsConnected() bool
	Publish(topic string, payload []byte) error
	Subscribe(topic string, handler func([]byte)) error
}

// MQTTClient implements the Client interface for MQTT
type MQTTClient struct {
	client MQTT.Client
}

func (m *MQTTClient) Connect() error {
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (m *MQTTClient) Disconnect(topic, payload string) error {
	if token := m.client.Publish(topic, mqttQoS["AtLeastOnce"], true, payload); token.Wait() && token.Error() != nil {
		fmt.Println("token error in Disconnect: ", token.Error())
	}
	m.client.Disconnect(250) // Graceful disconnect with timeout
	return nil
}

func (m *MQTTClient) IsConnected() bool {
	return m.client.IsConnectionOpen()
}

func (m *MQTTClient) Publish(topic string, payload []byte) error {
	token := m.client.Publish(topic, mqttQoS["AtLeastOnce"], true, payload)
	token.Wait()
	return token.Error()
}

func (m *MQTTClient) Subscribe(topic string, handler func([]byte)) error {
	token := m.client.Subscribe(topic, mqttQoS["AtLeastOnce"], func(client MQTT.Client, msg MQTT.Message) {
		handler(msg.Payload())
	})
	token.Wait()
	return token.Error()
}

// AzureIoTClient implements the Client interface for Azure IoT Hub
type AzureIoTClient struct {
	client *iotdevice.Client
}

func (a *AzureIoTClient) Connect() error {
	if err := a.client.Connect(context.Background()); err != nil {
		fmt.Println("Failed to connect azure IoT client: ", err)
	}
	return nil
}

func (a *AzureIoTClient) Disconnect(topic, payload string) error {
	if err := a.client.Close(); err != nil {
		return err
	}
	a.client = nil
	return nil
}

func (a *AzureIoTClient) IsConnected() bool {
	return a.client != nil
}

func (a *AzureIoTClient) Publish(topic string, payload []byte) error {
	parts := strings.Split(topic, "/")
	eventType := parts[len(parts)-1]
	return a.client.SendEvent(context.Background(), payload,
		iotdevice.WithSendExpiryTime(time.Now().Add(time.Minute)),
		iotdevice.WithSendProperties(map[string]string{
			eventType: "",
		}),
	)
}

func (a *AzureIoTClient) Subscribe(topic string, handler func([]byte)) error {
	go func() {
		ctx := context.Background()
		sub, err := a.client.SubscribeEvents(ctx)
		if err != nil {
			fmt.Println("Failed to subscribe azure IoT events: ", err)
			return
		}

		for msg := range sub.C() {
			handler(msg.Payload)
		}

	}()
	return nil
}
