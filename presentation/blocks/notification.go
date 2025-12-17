package blocks

// AddNotification は通知タイプに応じてメッセージに通知を追加する
func AddNotification(message, notificationType string) string {
	switch notificationType {
	case "here":
		return "<!here> " + message
	case "channel":
		return "<!channel> " + message
	case "none":
		return message
	default:
		return message
	}
}
