package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/amenzhinsky/iothub/iotdevice"
	iotmqtt "github.com/amenzhinsky/iothub/iotdevice/transport/mqtt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	UUID "github.com/google/uuid"
)

// Agent ...
type Agent interface {
	IsConnected() bool
	Connect() error
	Disconnect()
	SetOnConnectHandler(onConn OnConnectHandler)
	SetOnDisconnectHandler(onDisconn OnDisconnectHandler)
	SetOnMessageReceiveHandler(onMessageReceive OnMessageReceiveHandler)
	UploadConfig(action byte, edgeConfig EdgeConfig) bool
	SendDeviceStatus(status EdgeDeviceStatus) bool
	SendData(data EdgeData) bool
}

// Agent ...
type agent struct {
	options           EdgeAgentOptions
	client            Client // interface
	heartbeatTimer    chan bool
	dataRecoverTimer  chan bool
	dataRecoverHelper DataRecoverHelper
	cfgCache          configCache
	OnConnect         OnConnectHandler
	OnDisconnect      OnDisconnectHandler
	OnMessageReceive  OnMessageReceiveHandler
}

// OnConnectHandler ...
type OnConnectHandler func(Agent)

// OnDisconnectHandler ...
type OnDisconnectHandler func(Agent)

// OnMessageReceiveHandler ...
type OnMessageReceiveHandler func(MessageReceivedEventArgs, Agent)

// NewAgent ...
func NewAgent(options *EdgeAgentOptions) Agent {
	a := &agent{
		options:           *options,
		client:            nil,
		heartbeatTimer:    nil,
		dataRecoverTimer:  nil,
		dataRecoverHelper: nil,
		cfgCache:          newConfigCache(),
		OnConnect:         func(a Agent) {},
		OnDisconnect:      func(a Agent) {},
		OnMessageReceive:  func(res MessageReceivedEventArgs, a Agent) {},
	}
	if options.DataRecover {
		a.dataRecoverHelper = NewDataRecoverHelper(a.options.NodeID + "-" + dataRecoverFilePath)
	}

	// add cfg to memory from disk
	helper := newTagsCfgHelper()
	helper.getCfgFromFile(a, a.options.NodeID+tagsCfgFilePath)
	if err := a.newClient(); err != nil {
		log.Fatal(err)
	}
	return a
}

// IsConnected ...
func (a *agent) IsConnected() bool {
	if a.client == nil {
		return false
	}
	return a.client.IsConnected()
}

// Connect ...
func (a *agent) Connect() error {
	if err := a.client.Connect(); err != nil {
		return err
	}
	a.handleOnConnect()
	return nil
}

// Disconnect ...
func (a *agent) Disconnect() {
	if !a.client.IsConnected() {
		return
	}

	/* Send Disconnect message */
	topic := fmt.Sprintf(mqttTopic["DeviceConnTopic"], a.options.NodeID, a.options.DeviceID)
	if a.options.Type == EdgeType["GateWay"] {
		topic = fmt.Sprintf(mqttTopic["NodeConnTopic"], a.options.NodeID)
	}
	payload := newDisconnectMessage().getPayload()
	go a.handleDisconnect()
	a.client.Disconnect(topic, payload)
}

func (a *agent) UploadConfig(action byte, config EdgeConfig) bool {
	if !a.IsConnected() {
		return false
	}
	nodeID := a.options.NodeID
	helper := newTagsCfgHelper()

	var payload configMessage
	var result = false
	switch action {
	case Action["Create"]:
		result, payload = convertCreateorUpdateConfig(action, nodeID, config, a.options.HeartBeatInterval)
		helper.overwriteCfgCache(a, config)
	case Action["Update"]:
		result, payload = convertCreateorUpdateConfig(action, nodeID, config, a.options.HeartBeatInterval)
		helper.updateCfgCache(a, config)
	case Action["Delete"]:
		result, payload = convertDeleteConfig(action, nodeID, config)
		helper.deleteCfgCache(a, config)
	case Action["Delsert"]:
		result, payload = convertCreateorUpdateConfig(action, nodeID, config, a.options.HeartBeatInterval)
		helper.overwriteCfgCache(a, config)
	default:
		result = false
	}

	if result {
		topic := fmt.Sprintf(mqttTopic["ConfigTopic"], a.options.NodeID)
		if err := a.client.Publish(topic, []byte(payload.getPayload())); err != nil {
			fmt.Println("error in upload config: ", err)
			result = false
		}
	}

	helper.addCfgToFile(a, a.options.NodeID+tagsCfgFilePath)

	return result
}

func (a *agent) SendDeviceStatus(statuses EdgeDeviceStatus) bool {
	if !(a.IsConnected()) {
		return false
	}
	msg := newStatusMessage()
	msg.Ts = statuses.Timestamp.Format(time.RFC3339)
	for _, status := range statuses.DeviceList {
		msg.D.Dev[status.ID] = status.Status
	}
	payload := msg.getPayload()
	topic := fmt.Sprintf(mqttTopic["NodeConnTopic"], a.options.NodeID)
	if err := a.client.Publish(topic, []byte(payload)); err != nil {
		fmt.Println("error in send dvice status: ", err)
		return false
	}
	return true
}

func (a *agent) SendData(data EdgeData) bool {
	result, payloads := convertTagValue(data, a)
	topic := fmt.Sprintf(mqttTopic["DataTopic"], a.options.NodeID)
	if !a.IsConnected() {
		for _, payload := range payloads {
			if a.dataRecoverHelper != nil {
				a.dataRecoverHelper.Write(payload)
			}
		}
		result = false
	} else {
		for _, payload := range payloads {
			if err := a.client.Publish(topic, []byte(payload)); err != nil {
				fmt.Println("error in send data: ", err)
				if a.dataRecoverHelper != nil {
					a.dataRecoverHelper.Write(payload)
				}
				result = false
			}
		}
	}
	return result
}

func (a *agent) getCredentailFromDCCS() error {
	url := a.options.DCCS.URL
	if url[len(url)-1:] == "/" {
		a.options.DCCS.URL = url[:len(url)-1]
	}
	url = fmt.Sprintf("%s/v1/serviceCredentials/%s", a.options.DCCS.URL, a.options.DCCS.Key)
	res, error := http.Get(url)
	if error != nil {
		return error
	}

	body, error := io.ReadAll(res.Body)
	if error != nil {
		return error
	}

	var response struct {
		ServiceName string
		ServiceHost string
		Credential  struct {
			Password  string
			Username  string
			Protocols map[string]struct {
				Ssl      bool
				Username string
				Password string
				Port     int
			}
		}
	}
	error = json.Unmarshal([]byte(body), &response)
	if error != nil {
		return error
	}

	a.options.MQTT.HostName = response.ServiceHost
	if a.options.UseSecure {
		a.options.MQTT.Port = response.Credential.Protocols["mqtt+ssl"].Port
		a.options.MQTT.UserName = response.Credential.Protocols["mqtt+ssl"].Username
		a.options.MQTT.Password = response.Credential.Protocols["mqtt+ssl"].Password
		a.options.MQTT.ProtocalType = Protocol["TLS"]
	} else {
		a.options.MQTT.Port = response.Credential.Protocols["mqtt"].Port
		a.options.MQTT.UserName = response.Credential.Protocols["mqtt"].Username
		a.options.MQTT.Password = response.Credential.Protocols["mqtt"].Password
	}
	return nil
}

func (a *agent) newClient() error {
	switch a.options.ConnectType {
	case "MQTT", "DCCS":
		if a.options.ConnectType == ConnectType["DCCS"] {
			if !a.options.DCCS.isValid() {
				return fmt.Errorf("DCCS options is invalid")
			}
			error := a.getCredentailFromDCCS()
			if error != nil {
				fmt.Println(error)
				return error
			}
		}
		if !a.options.MQTT.isValid() {
			return fmt.Errorf("MQTT options is invalid")
		}

		clientOptions, _ := a.newMQTTOptions()
		mqttClient := MQTT.NewClient(clientOptions)
		a.client = &MQTTClient{client: mqttClient}
	case "AzureIoTHub":
		c, err := iotdevice.NewFromConnectionString(iotmqtt.New(), a.options.AzureIoTHubOptions.ConnectionString)
		if err != nil {
			return err
		}
		a.client = &AzureIoTClient{
			client: c,
		}
	default:
		return fmt.Errorf("unsupported ConnectType: %s", a.options.ConnectType)
	}
	return nil
}

func (a *agent) newMQTTOptions() (*MQTT.ClientOptions, error) {
	clientOptions := MQTT.NewClientOptions()
	schema := protocolScheme[Protocol["TCP"]]
	// Enable Debug
	// MQTT.DEBUG = log.New(os.Stdout, "[Debug] ", 0)
	// MQTT.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	// MQTT.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
	// MQTT.WARN = log.New(os.Stdout, "[WARN]  ", 0)

	if a.options.MQTT.ProtocalType == Protocol["WebSocket"] {
		schema = protocolScheme[Protocol["WebSocket"]]
	}
	if a.options.MQTT.ProtocalType == Protocol["TLS"] {
		schema = protocolScheme[Protocol["TLS"]]
	}

	server := fmt.Sprintf("%s://%s:%d", schema, a.options.MQTT.HostName, a.options.MQTT.Port)
	clientOptions.AddBroker(server)
	uuid := UUID.New()
	clientOptions.SetClientID(fmt.Sprintf("EdgeAgent_%s", uuid))
	clientOptions.SetAutoReconnect(true)
	clientOptions.SetConnectRetry(true)
	clientOptions.SetConnectRetryInterval(time.Duration(a.options.ReconnectInterval) * time.Second)
	clientOptions.SetCleanSession(false)
	clientOptions.SetPassword(a.options.MQTT.Password)
	clientOptions.SetUsername(a.options.MQTT.UserName)
	clientOptions.SetMaxReconnectInterval(time.Duration(a.options.ReconnectInterval) * time.Second)
	topic := fmt.Sprintf(mqttTopic["NodeConnTopic"], a.options.NodeID)
	payload := newWillMessage().getPayload()
	clientOptions.SetWill(topic, payload, mqttQoS["AtLeastOnce"], true)
	clientOptions.SetConnectionLostHandler(func(c MQTT.Client, err error) {
		fmt.Println("Reconnecting...")
		if err != nil {
			fmt.Println(err)
		}
		if a.options.ConnectType == ConnectType["DCCS"] {
			error := a.getCredentailFromDCCS()
			if error != nil {
				fmt.Println(err)
			}
		}
		go a.OnDisconnect(a)
	})
	return clientOptions, nil
}

func (a *agent) SetOnConnectHandler(onConn OnConnectHandler) {
	a.OnConnect = onConn
}

func (a *agent) SetOnDisconnectHandler(onDisconn OnDisconnectHandler) {
	a.OnDisconnect = onDisconn
}

func (a *agent) SetOnMessageReceiveHandler(onMessageReceive OnMessageReceiveHandler) {
	a.OnMessageReceive = onMessageReceive
}

func (a *agent) handleOnConnect() {
	/* subscribe */
	cmdTopic := fmt.Sprintf(mqttTopic["DeviceCmdTopic"], a.options.NodeID, a.options.DeviceID)
	if a.options.Type == EdgeType["Gateway"] {
		cmdTopic = fmt.Sprintf(mqttTopic["NodeCmdTopic"], a.options.NodeID)
	}
	if err := a.client.Subscribe(cmdTopic, a.handleCmdReceive); err != nil {
		fmt.Println("Failed to subscribe Cmd: ", err)
	}
	ackTopic := fmt.Sprintf(mqttTopic["AckTopic"], a.options.NodeID)
	if err := a.client.Subscribe(ackTopic, a.handleAckReceive); err != nil {
		fmt.Println("Failed to subscribe Ack: ", err)
	}

	/* Send connect Message */
	topic := fmt.Sprintf(mqttTopic["DeviceConnTopic"], a.options.NodeID, a.options.DeviceID)
	if a.options.Type == EdgeType["GateWay"] {
		topic = fmt.Sprintf(mqttTopic["NodeConnTopic"], a.options.NodeID)
	}
	payload := newConnMessage().getPayload()
	if err := a.client.Publish(topic, []byte(payload)); err != nil {
		fmt.Println("Failed to publish Conn: ", err)
	}

	/* heartbeat */
	if a.options.HeartBeatInterval > 0 && a.heartbeatTimer == nil {
		interval := a.options.HeartBeatInterval
		a.heartbeatTimer = setInterval(a.sendHeartBeat, interval, true)
	}

	/* Recover */
	if a.options.DataRecover && a.dataRecoverTimer == nil {
		a.dataRecoverTimer = setInterval(a.sendRecover, dataRecoverInterval, true)
	}

	go a.OnConnect(a)
}

func (a *agent) handleDisconnect() {
	for a.client.IsConnected() {
	}
	fmt.Println("Disconnected...")
	a.client = nil
	a.heartbeatTimer <- false
	a.heartbeatTimer = nil
	a.dataRecoverTimer <- false
	a.dataRecoverTimer = nil
	go a.OnDisconnect(a)
}

func (a *agent) handleCmdReceive(msg []byte) {
	payload := string(msg)
	if !isJSON(payload) {
		fmt.Println("Invalid JSON format")
		return
	}
	var data cmdMessage
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		fmt.Println("Cmd decode failed:", err)
	}
	var message interface{}
	var argType byte
	switch data.D.Cmd {
	case "WV":
		argType = MessageType["WriteValue"]
		message = getWriteDataMessageFromCmdMessage(data.D.Val, data.Ts)
	case "TSyn":
		argType = MessageType["TimeSync"]
		message = getTimeSyncMessageFromCmdMessage(data.D.UTC)
	default:
		// fmt.Println("Message format is invalid")
		return
	}
	res := MessageReceivedEventArgs{
		Type:    argType,
		Message: message,
	}
	go a.OnMessageReceive(res, a)
}

func (a *agent) handleAckReceive(msg []byte) {
	payload := string(msg)
	if !isJSON(payload) {
		fmt.Println("Invalid JSON format")
		return
	}
	var data ackConfigMessage
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		fmt.Println("Ack decode failed:", err)
		return
	}
	val, ok := data.D.Cfg.(float64)
	if data.D.Cfg == nil || !ok {
		// fmt.Println("Message format is invalid")
		return
	}
	var result = false
	if val > 0 {
		result = true
	}
	message := ConfigAckMessage{
		Result: result,
	}
	res := MessageReceivedEventArgs{
		Type:    MessageType["ConfigAck"],
		Message: message,
	}
	go a.OnMessageReceive(res, a)
}

func (a *agent) sendHeartBeat() {
	if !a.IsConnected() {
		return
	}
	topic := fmt.Sprintf(mqttTopic["DeviceConnTopic"], a.options.NodeID, a.options.DeviceID)
	if a.options.Type == EdgeType["GateWay"] {
		topic = fmt.Sprintf(mqttTopic["NodeConnTopic"], a.options.NodeID)
	}
	payload := newHeartBeatMessage().getPayload()
	if err := a.client.Publish(topic, []byte(payload)); err != nil {
		fmt.Println("error in sendHeartBeat: ", err)
	}
}

func (a *agent) sendRecover() {
	if !a.IsConnected() {
		return
	}
	if a.dataRecoverHelper == nil {
		return
	}
	helper := a.dataRecoverHelper

	if !helper.IsDataExist() {
		return
	}
	messages := helper.Read(defaultReadRecordCount)
	topic := fmt.Sprintf(mqttTopic["DataTopic"], a.options.NodeID)
	for _, message := range messages {
		if !a.IsConnected() {
			helper.Write(message)
			continue
		}
		if err := a.client.Publish(topic, []byte(message)); err != nil {
			fmt.Println("error in sendRecover: ", err)
			helper.Write(message)
		}
	}
}
