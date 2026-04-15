package domain

// messagetypeconstant
const (
	MessageTypeHello  = "hello"  // 握手message
	MessageTypeAbort  = "abort"  // in止message
	MessageTypeListen = "listen" // listenmessage
	MessageTypeIot    = "iot"    // 物联网message
)

// servermessagetypeconstant
const (
	ServerMessageTypeHello = "hello" // 握手message
	ServerMessageTypeStt   = "stt"   // voice转text
	ServerMessageTypeTts   = "tts"   // text转voice
	ServerMessageTypeIot   = "iot"   // 物联网message
	ServerMessageTypeLlm   = "llm"   // largelanguagemodel
	ServerMessageTypeText  = "text"  // textmessage
)

// messagestateconstant
const (
	MessageStateStart   = "start"   // startstate
	MessageStateStop    = "stop"    // stopstate
	MessageStateDetect  = "detect"  // detectstate
	MessageStateAbort   = "abort"   // in止state
	MessageStateSuccess = "success" // successfulstate
)
