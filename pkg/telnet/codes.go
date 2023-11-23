package telnet

const (
	CR = byte('\r')
	LF = byte('\n')
)

// Command Codes
const (
	cmd_XEOF  = 236 // End of file: EOF is already used
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

// Options Codes
const (
	opt_BINARY           = 0   // Binary Transmission - RFC 856
	opt_ECHO             = 1   // Echo                - RFC 857
	opt_RCP              = 2   // Reconnection
	opt_SGA              = 3   // Suppress Go Ahead   - RFC 858
	opt_NAMS             = 4   // Approx Message Size Negotiation
	opt_STATUS           = 5   // Status              - RFC 859
	opt_TM               = 6   // Timing Mark         - RFC 860
	opt_RCTE             = 7   // Remote controlled transmission and echo - RFC 563,726
	opt_NAOL             = 8   // Negotiate about output line width - NIC50005
	opt_NAOP             = 9   // Negotiate about output page size - NIC50005
	opt_NAOCRD           = 10  // Negotiate about CR disposition - RFC 652
	opt_NAOHTS           = 11  // Negotiate about horizontal tabstops - RFC 653
	opt_NAOHTD           = 12  // Negotiate about horizontal tab disposition - RFC 654
	opt_NAOFFD           = 13  // Negotiate about formfeed disposition - RFC 655
	opt_NAOVTS           = 14  // Negotiate about vertical tab stops - RFC 656
	opt_NAOVTD           = 15  // Negotiate about vertical tab disposition - RFC 657
	opt_NAOLFD           = 16  // Negotiate about output LF disposition - RFC 658
	opt_XASCII           = 17  // Extended ascii character set - RFC 698
	opt_LOGOUT           = 18  // Force logout        - RFC 727
	opt_BM               = 19  // Byte Macro          - RFC 735
	opt_DET              = 20  // Data Entry Terminal - RFC 732,1043
	opt_SUPDUP           = 21  // SUPDUP Protocol    - RFC 734,736
	opt_SUPDUPOUTPUT     = 22  // SUPDUP Output      - RFC 749
	opt_SNDLOC           = 23  // Send Location      - RFC 779
	opt_TTYPE            = 24  // Terminal Type      - RFC 1091
	opt_EOR              = 25  // End of Record      - RFC 885
	opt_TUID             = 26  // TACACS User Identification - RFC 927
	opt_OUTMRK           = 27  // Output Marking     - RFC 933
	opt_TTYLOC           = 28  // Terminal Location Number - RFC 946
	opt_3270REGIME       = 29  // Telnet 3270 Regime - RFC 1041
	opt_X3PAD            = 30  // X.3 PAD            - RFC 1053
	opt_NAWS             = 31  // Negotiate window size - RFC 1073
	opt_TSPEED           = 32  // Terminal Speed     - RFC 1079
	opt_LFLOW            = 33  // Remote Flow Control - RFC 1372
	opt_LINEMODE         = 34  // Line mode option     - RFC 1184
	opt_XDISPLOC         = 35  // X Display Location - RFC 1096
	opt_OLD_ENVIRON      = 36  // Environment Option - RFC 1408
	opt_AUTHENTICATION   = 37  // Authenticate - RFC 1416,2941,2942,2943,2951
	opt_ENCRYPT          = 38  // Encryption Option - RFC 2946
	opt_NEW_ENVIRON      = 39  // New Environment Option - RFC 1572
	opt_TN3270E          = 40  // TN3270 enhancements    - RFC 2355
	opt_XAUTH            = 41  // XAUTH
	opt_CHARSET          = 42  // Negotiate charset to use - RFC 2066
	opt_RSP              = 43  // Telnet remote serial port
	opt_COM_PORT_OPTION  = 44  // Com port control option - RFC 2217
	opt_SLE              = 45  // Telnet suppress local echo
	opt_STARTTLS         = 46  // Telnet Start TLS
	opt_KERMIT           = 47  // Automatic Kermit file transfer - RFC 2840
	opt_SEND_URL         = 48  // Send URL
	opt_FORWARD_X        = 49  // X forwarding
	opt_MCCP1            = 85  // Mud Compression Protocol (v1)
	opt_MCCP2            = 86  // Mud Compression Protocol (v2)
	opt_MSP              = 90  // Mud Sound Protocol
	opt_MXP              = 91  // Mud Extension Protocol
	opt_ZMP              = 93  // Zenith Mud Protocol
	opt_PRAGMA_LOGON     = 138 // Telnet option pragma logon
	opt_SSPI_LOGON       = 139 // Telnet option SSPI login
	opt_PRAGMA_HEARTBEAT = 140 // Telnet option pragma heartbeat
	opt_GMCP             = 201 // Generic Mud Communication Protocol
	opt_EXOPL            = 255 // extended-options-list - RFC 861
)
