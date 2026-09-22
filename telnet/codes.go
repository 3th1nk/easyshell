package telnet

// telnet 协议常量(不属于公共 API，全部私有)

const (
	cr = byte('\r')
	lf = byte('\n')
)

// Command Codes
const (
	cmd_SUSP  = 237 // Suspend process
	cmd_ABORT = 238 // Abort process
	cmd_EOR   = 239 // end of record (transparent mode)
	cmd_SE    = 240 // end sub negotiation
	cmd_NOP   = 241 // nop
	cmd_DM    = 242 // data mark--for connect. cleaning
	cmd_BREAK = 243 // break
	cmd_IP    = 244 // interrupt process--permanently
	cmd_AO    = 245 // abort output--but let prog finish
	cmd_AYT   = 246 // are you there
	cmd_EC    = 247 // erase the current character
	cmd_EL    = 248 // erase the current line
	cmd_GA    = 249 // you may reverse the line
	cmd_SB    = 250 // interpret as sub negotiation
	cmd_WILL  = 251 // I will use option
	cmd_WONT  = 252 // I won't use option
	cmd_DO    = 253 // please, you use option
	cmd_DONT  = 254 // you are not to use option
	cmd_IAC   = 255 // interpret as command
)

// Options Codes(仅保留本实现使用的)
const (
	opt_ECHO  = 1  // Echo                - RFC 857
	opt_SGA   = 3  // Suppress Go Ahead   - RFC 858
	opt_TTYPE = 24 // Terminal Type       - RFC 1091
	opt_NAWS  = 31 // Negotiate window size - RFC 1073
)
