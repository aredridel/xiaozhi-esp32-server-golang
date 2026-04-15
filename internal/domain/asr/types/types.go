package types

const (
	EmptyReasonNone               = ""
	EmptyReasonNoServerResponse   = "no_server_response"
	EmptyReasonProviderEmptyFinal = "provider_empty_final"

	RetryReasonNone                           = ""
	RetryReasonDoubaoResponseCode45000081     = "doubao_response_code_45000081"
	RetryReasonDoubaoWaitingNextPacketTimeout = "doubao_waiting_next_packet_timeout"
	RetryReasonXunfeiServiceInstanceInvalid   = "xunfei_service_instance_invalid"
	RetryReasonAliyunQwen3ConnectionClosed    = "aliyun_qwen3_connection_closed"
)

// StreamingResult streaming recognizeresult
type StreamingResult struct {
	Text        string // recognizeoftext
	IsFinal     bool   // whetherisfinallyresult
	Error       error  // errorinfo
	AsrType     string // asr type
	Mode        string // pattern
	EmptyReason string // emptyresultreason，onlyat Text isemptywhenused for区minuteup游emptyresult/empty转
	RetryReason string // 可recoveryerrorreason，onlyatneedreleasecurrentresourceandretrywhenuse
}
