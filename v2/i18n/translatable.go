package i18n

type Translatable interface {
	T(provider MessageProvider) string
}

type TranslatableMessage struct {
	MsgOrKey string
}

func NewTranslatable(msgOrKey string) *TranslatableMessage {
	return &TranslatableMessage{
		MsgOrKey: msgOrKey,
	}
}

func (t *TranslatableMessage) T(provider MessageProvider) string {
	return provider.GetMessage(t.MsgOrKey)
}
