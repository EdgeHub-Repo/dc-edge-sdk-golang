package agent

import (
	"fmt"
	"time"
)

// EdgeAgentOptions ...
type EdgeAgentOptions struct {
	ReconnectInterval  int // second
	NodeID             string
	DeviceID           string
	Type               byte
	HeartBeatInterval  int
	DataRecover        bool
	ConnectType        string
	UseSecure          bool
	MQTT               *MQTTOptions
	DCCS               *DCCSOptions
	AzureIoTHubOptions *AzureIoTHubOptions
}

// MQTTOptions ...
type MQTTOptions struct {
	HostName     string
	Port         int
	UserName     string
	Password     string
	ProtocalType string
}

// DCCSOptions ...
type DCCSOptions struct {
	URL string
	Key string
}

// AzureIoTHub Options ...
type AzureIoTHubOptions struct {
	ConnectionString string
}

// DeviceStatus ...
type DeviceStatus struct {
	ID     string
	Status byte
}

// EdgeConfig ...
type EdgeConfig struct {
	Node NodeConfig
}

// EdgeData ...
type EdgeData struct {
	TagList   []EdgeTag
	Timestamp time.Time
}

// EdgeTag ...
type EdgeTag struct {
	DeviceID string
	TagName  string
	Value    interface{}
}

// EdgeDeviceStatus ...
type EdgeDeviceStatus struct {
	DeviceList []DeviceStatus
	Timestamp  time.Time
}

// NodeConfig ...
type NodeConfig struct {
	primaryIP   interface{}
	backupIP    interface{}
	primaryPort interface{}
	backupPort  interface{}
	nodeType    interface{}
	DeviceList  []DeviceConfig
}

// DeviceConfig ...
type DeviceConfig struct {
	id                  interface{}
	name                interface{}
	comPortNumber       interface{}
	deviceType          interface{}
	description         interface{}
	ip                  interface{}
	port                interface{}
	retentionPolicyName interface{}
	AnalogTagList       []AnalogTagConfig
	DiscreteTagList     []DiscreteTagConfig
	TextTagList         []TextTagConfig
}

// AnalogTagConfig ...
type AnalogTagConfig struct {
	name                  interface{}
	description           interface{}
	readOnly              interface{}
	arraySize             interface{}
	spanHigh              interface{}
	spanLow               interface{}
	engineerUnit          interface{}
	integerDisplayFormat  interface{}
	fractionDisplayFormat interface{}
	sendWhenValueChanged  interface{}
	blockType             interface{}
}

// DiscreteTagConfig ...
type DiscreteTagConfig struct {
	name                 interface{}
	description          interface{}
	readOnly             interface{}
	arraySize            interface{}
	state0               interface{}
	state1               interface{}
	state2               interface{}
	state3               interface{}
	state4               interface{}
	state5               interface{}
	state6               interface{}
	state7               interface{}
	sendWhenValueChanged interface{}
	blockType            interface{}
}

// TextTagConfig ...
type TextTagConfig struct {
	name                 interface{}
	description          interface{}
	readOnly             interface{}
	arraySize            interface{}
	sendWhenValueChanged interface{}
	blockType            interface{}
}

// MessageReceivedEventArgs ...
type MessageReceivedEventArgs struct {
	Type    byte
	Message interface{}
}

// ConfigAckMessage ...
type ConfigAckMessage struct {
	Result bool
}

// WriteDataMessage ...
type WriteDataMessage struct {
	DeviceList []Device
	Timestamp  time.Time
}

// Device ...
type Device struct {
	ID      string
	TagList []Tag
}

// Tag ...
type Tag struct {
	Name  string
	Value interface{}
}

// TimeSyncMessage ...
type TimeSyncMessage struct {
	UTCTime time.Time
}

// BlockConfig ...
type BlockConfig struct {
	BlockType       string
	AnalogTagList   []AnalogTagConfig
	DiscreteTagList []DiscreteTagConfig
	TextTagList     []TextTagConfig
}

func NewBlockConfig(blockType string) BlockConfig {
	return BlockConfig{
		BlockType: blockType,
	}
}

// NewEdgeAgentOptions will create a new EdgeAgentOption with some new values
//
//	ReconnectInterval: 1
//	Type: EdgeType["Gateway"]
//	HeartBeatInterval: HeartBeatInterval
//	DataRecover: true,
//	ConnectType: ConnectType["DCCS"],
//	UseSecure: false,
//	MQTT.Port: 1883
//	MQTT.ProtocalType: Protocol["TCP"]
func NewEdgeAgentOptions() *EdgeAgentOptions {
	options := &EdgeAgentOptions{
		ReconnectInterval: 1,
		NodeID:            "",
		DeviceID:          "",
		Type:              EdgeType["Gateway"],
		HeartBeatInterval: HeartBeatInterval,
		DataRecover:       true,
		ConnectType:       ConnectType["DCCS"],
		UseSecure:         false,
		MQTT: &MQTTOptions{
			HostName:     "",
			Port:         1883,
			UserName:     "",
			Password:     "",
			ProtocalType: Protocol["TCP"],
		},
		DCCS: &DCCSOptions{
			URL: "https://api-dccs.wise-paas.com/",
			Key: "0c053cf0329e0100c5255cfdd55defcz",
		},
		AzureIoTHubOptions: &AzureIoTHubOptions{
			ConnectionString: "",
		},
	}
	return options
}

// NewNodeConfig ...
func NewNodeConfig() NodeConfig {
	return NodeConfig{}
}

// NewDeviceConfig ...
func NewDeviceConfig(ID string) DeviceConfig {
	return DeviceConfig{
		id: ID,
	}
}

// NewAnaglogTagConfig ...
func NewAnaglogTagConfig(name string) AnalogTagConfig {
	return AnalogTagConfig{
		name:                 name,
		sendWhenValueChanged: false,
	}
}

// NewDiscreteTagConfig ...
func NewDiscreteTagConfig(name string) DiscreteTagConfig {
	return DiscreteTagConfig{
		name:                 name,
		sendWhenValueChanged: false,
	}
}

// NewTextTagConfig ...
func NewTextTagConfig(name string) TextTagConfig {
	return TextTagConfig{
		name:                 name,
		sendWhenValueChanged: false,
	}
}

func getWriteDataMessageFromCmdMessage(data interface{}, ts_string string) WriteDataMessage {
	m := data.(map[string]interface{})

	layout := "2006-01-02T15:04:05.000Z"
	ts, err := time.Parse(layout, ts_string)

	if err != nil {
		ts = time.Now()
		fmt.Println(err)
	}

	message := WriteDataMessage{
		Timestamp: ts,
	}
	for device, value := range m {
		d := Device{
			ID: device,
		}
		tagList := value.(map[string]interface{})
		for tag, v := range tagList {
			t := Tag{
				Name:  tag,
				Value: v,
			}
			d.TagList = append(d.TagList, t)
		}
		message.DeviceList = append(message.DeviceList, d)
	}

	return message
}

func getTimeSyncMessageFromCmdMessage(utc int) TimeSyncMessage {
	ts := time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC)
	duration := time.Duration(utc) * time.Second
	message := TimeSyncMessage{
		UTCTime: ts.Add(duration),
	}
	return message
}

func (o *MQTTOptions) isValid() bool {
	return !(o.HostName == "" || o.Port == 0 || o.ProtocalType == "")
}

func (o *DCCSOptions) isValid() bool {
	return !(o.URL == "" || o.Key == "")
}

// SetType ...
func (config *NodeConfig) SetType(t byte) {
	config.nodeType = t
}

// SetName ...
func (config *DeviceConfig) SetName(name string) {
	config.name = name
}

// SetType ...
func (config *DeviceConfig) SetType(deviceType string) {
	config.deviceType = deviceType
}

// SetDescription ...
func (config *DeviceConfig) SetDescription(desc string) {
	config.description = desc
}

// SetRetentionPolicyName ...
func (config *DeviceConfig) SetRetentionPolicyName(name string) {
	config.retentionPolicyName = name
}

func (config *DeviceConfig) AddBlock(name string, blockConfig BlockConfig) {
	for _, analogTag := range blockConfig.AnalogTagList {
		analogTag.SetBlockType(blockConfig.BlockType)
		analogTag.name = fmt.Sprintf("%s:%s", name, analogTag.name)
		config.AnalogTagList = append(config.AnalogTagList, analogTag)
	}
	for _, discreteTag := range blockConfig.DiscreteTagList {
		discreteTag.SetBlockType(blockConfig.BlockType)
		discreteTag.name = fmt.Sprintf("%s:%s", name, discreteTag.name)
		config.DiscreteTagList = append(config.DiscreteTagList, discreteTag)
	}
	for _, textTag := range blockConfig.TextTagList {
		textTag.SetBlockType(blockConfig.BlockType)
		textTag.name = fmt.Sprintf("%s:%s", name, textTag.name)
		config.TextTagList = append(config.TextTagList, textTag)
	}

}

// SetDescription ...
func (config *AnalogTagConfig) SetDescription(desc string) {
	config.description = desc
}

// SetReadOnly ...
func (config *AnalogTagConfig) SetReadOnly(readOnly bool) {
	config.readOnly = readOnly
}

// SetArraySize ...
func (config *AnalogTagConfig) SetArraySize(num uint) {
	config.arraySize = num
}

// SetSpanHigh ...
func (config *AnalogTagConfig) SetSpanHigh(high float64) {
	config.spanHigh = high
}

// SetSpanLow ...
func (config *AnalogTagConfig) SetSpanLow(low float64) {
	config.spanLow = low
}

// SetEngineerUnit ...
func (config *AnalogTagConfig) SetEngineerUnit(unit string) {
	config.engineerUnit = unit
}

// SetIntegerDisplayFormat ...
func (config *AnalogTagConfig) SetIntegerDisplayFormat(num uint) {
	config.integerDisplayFormat = num
}

// SetFractionDisplayFormat ...
func (config *AnalogTagConfig) SetFractionDisplayFormat(num uint) {
	config.fractionDisplayFormat = num
}

// SetSendWhenValueChanged ...
func (config *AnalogTagConfig) SetSendWhenValueChanged(flag bool) {
	config.sendWhenValueChanged = flag
}

// SetBlockType ...
func (config *AnalogTagConfig) SetBlockType(blockType string) {
	config.blockType = blockType
}

// SetDescription ...
func (config *DiscreteTagConfig) SetDescription(desc string) {
	config.description = desc
}

// SetReadOnly ...
func (config *DiscreteTagConfig) SetReadOnly(readOnly bool) {
	config.readOnly = readOnly
}

// SetArraySize ...
func (config *DiscreteTagConfig) SetArraySize(num uint) {
	config.arraySize = num
}

// SetState0 ...
func (config *DiscreteTagConfig) SetState0(state string) {
	config.state0 = state
}

// SetState1 ...
func (config *DiscreteTagConfig) SetState1(state string) {
	config.state1 = state
}

// SetState2 ...
func (config *DiscreteTagConfig) SetState2(state string) {
	config.state2 = state
}

// SetState3 ...
func (config *DiscreteTagConfig) SetState3(state string) {
	config.state3 = state
}

// SetState4 ...
func (config *DiscreteTagConfig) SetState4(state string) {
	config.state4 = state
}

// SetState5 ...
func (config *DiscreteTagConfig) SetState5(state string) {
	config.state5 = state
}

// SetState6 ...
func (config *DiscreteTagConfig) SetState6(state string) {
	config.state6 = state
}

// SetState7 ...
func (config *DiscreteTagConfig) SetState7(state string) {
	config.state7 = state
}

// SetSendWhenValueChanged ...
func (config *DiscreteTagConfig) SetSendWhenValueChanged(flag bool) {
	config.sendWhenValueChanged = flag
}

// SetBlockType ...
func (config *DiscreteTagConfig) SetBlockType(blockType string) {
	config.blockType = blockType
}

// SetDescription ...
func (config *TextTagConfig) SetDescription(desc string) {
	config.description = desc
}

// SetReadOnly ...
func (config *TextTagConfig) SetReadOnly(readOnly bool) {
	config.readOnly = readOnly
}

// SetArraySize ...
func (config *TextTagConfig) SetArraySize(num uint) {
	config.arraySize = num
}

// SetSendWhenValueChanged ...
func (config *TextTagConfig) SetSendWhenValueChanged(flag bool) {
	config.sendWhenValueChanged = flag
}

// SetBlockType ...
func (config *TextTagConfig) SetBlockType(blockType string) {
	config.blockType = blockType
}
