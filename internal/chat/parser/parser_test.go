package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_WhatsApp(t *testing.T) {
	p, err := New("whatsapp")
	require.NoError(t, err)
	assert.NotNil(t, p)
	_, ok := p.(*WhatsAppParser)
	assert.True(t, ok, "expected *WhatsAppParser for platform 'whatsapp'")
}

func TestNew_Facebook(t *testing.T) {
	p, err := New("facebook")
	require.NoError(t, err)
	assert.NotNil(t, p)
	_, ok := p.(*MetaParser)
	assert.True(t, ok, "expected *MetaParser for platform 'facebook'")
}

func TestNew_Instagram(t *testing.T) {
	p, err := New("instagram")
	require.NoError(t, err)
	assert.NotNil(t, p)
	_, ok := p.(*MetaParser)
	assert.True(t, ok, "expected *MetaParser for platform 'instagram'")
}

func TestNew_UnsupportedPlatform(t *testing.T) {
	platforms := []string{"telegram", "viber", "tiktok", "", "WHATSAPP", "WhatsApp"}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			p, err := New(platform)
			assert.Nil(t, p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported platform")
		})
	}
}

func TestNew_ReturnsWebhookParserInterface(t *testing.T) {
	// Verify the returned value satisfies the WebhookParser interface.
	// This is enforced at compile time by the type system, but we make it
	// explicit here for documentation purposes.
	p, err := New("whatsapp")
	require.NoError(t, err)

	var _ WebhookParser = p
}
