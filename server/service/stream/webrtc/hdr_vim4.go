//go:build linux && vim4

package webrtc

import "NanoKVM-Server/common"

func h265Fmtp() string {
	if common.H265Main10() {
		return "profile-id=2;tier-flag=0;level-id=180;tx-mode=SRST"
	}
	return "profile-id=1;tier-flag=0;level-id=180;tx-mode=SRST"
}
