package i18n

import "fmt"

// Messages is one locale's catalog. Every field is a user-visible string, and
// every locale fills every field.
type Messages struct {
	AppName string

	MagicLinkSubject string
	// MagicLinkBody takes the link and its lifetime in minutes.
	MagicLinkBody string
}

func (m Messages) MagicLinkText(link string, minutes int) string {
	return fmt.Sprintf(m.MagicLinkBody, link, minutes)
}

var en = Messages{
	AppName:          "Briefing",
	MagicLinkSubject: "Your Briefing sign-in link",
	MagicLinkBody: "Open this link to sign in to Briefing:\n\n%[1]s\n\n" +
		"The link works once and expires in %[2]d minutes. If you did not ask for it, ignore this email.\n",
}

var fr = Messages{
	AppName:          "Briefing",
	MagicLinkSubject: "Votre lien de connexion à Briefing",
	MagicLinkBody: "Ouvrez ce lien pour vous connecter à Briefing :\n\n%[1]s\n\n" +
		"Le lien ne fonctionne qu'une fois et expire dans %[2]d minutes. Si vous ne l'avez pas demandé, ignorez cet e-mail.\n",
}
