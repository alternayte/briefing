package i18n

import "fmt"

// Messages is one locale's catalog. Every field is a user-visible string, and
// every locale fills every field.
type Messages struct {
	AppName string

	MagicLinkSubject string
	// MagicLinkBody takes the link and its lifetime in minutes.
	MagicLinkBody string

	SkipToContent string
	SignOut       string

	LoginTitle             string
	LoginIntro             string
	LoginGitHub            string
	LoginOr                string
	LoginEmailLabel        string
	LoginEmailSend         string
	LoginLinkSent          string
	LoginErrorNotAllowed   string
	LoginErrorExpired      string
	LoginErrorRateLimited  string
	LoginErrorInvalidEmail string
	LoginErrorGeneric      string

	VerifyTitle  string
	VerifyBody   string
	VerifyButton string
}

func (m Messages) MagicLinkText(link string, minutes int) string {
	return fmt.Sprintf(m.MagicLinkBody, link, minutes)
}

var en = Messages{
	AppName:          "Briefing",
	MagicLinkSubject: "Your Briefing sign-in link",
	MagicLinkBody: "Open this link to sign in to Briefing:\n\n%[1]s\n\n" +
		"The link works once and expires in %[2]d minutes. If you did not ask for it, ignore this email.\n",

	SkipToContent: "Skip to content",
	SignOut:       "Sign out",

	LoginTitle:             "Sign in",
	LoginIntro:             "Briefing reads the tech and science news, keeps what matters, and helps you remember it.",
	LoginGitHub:            "Continue with GitHub",
	LoginOr:                "or",
	LoginEmailLabel:        "Email address",
	LoginEmailSend:         "Email me a sign-in link",
	LoginLinkSent:          "Check your inbox. If this address can sign in, a link is on its way.",
	LoginErrorNotAllowed:   "This address is not on the invite list of this Briefing.",
	LoginErrorExpired:      "That sign-in link has expired or was already used. Ask for a new one.",
	LoginErrorRateLimited:  "Too many attempts. Wait a few minutes, then try again.",
	LoginErrorInvalidEmail: "Enter a valid email address.",
	LoginErrorGeneric:      "Sign-in did not work. Try again.",

	VerifyTitle:  "Confirm sign-in",
	VerifyBody:   "Select the button to finish signing in. If you did not ask for this link, close this page.",
	VerifyButton: "Sign in",
}

var fr = Messages{
	AppName:          "Briefing",
	MagicLinkSubject: "Votre lien de connexion à Briefing",
	MagicLinkBody: "Ouvrez ce lien pour vous connecter à Briefing :\n\n%[1]s\n\n" +
		"Le lien ne fonctionne qu'une fois et expire dans %[2]d minutes. Si vous ne l'avez pas demandé, ignorez cet e-mail.\n",

	SkipToContent: "Aller au contenu",
	SignOut:       "Se déconnecter",

	LoginTitle:             "Connexion",
	LoginIntro:             "Briefing lit l'actualité tech et scientifique, garde l'essentiel et vous aide à le retenir.",
	LoginGitHub:            "Continuer avec GitHub",
	LoginOr:                "ou",
	LoginEmailLabel:        "Adresse e-mail",
	LoginEmailSend:         "Recevoir un lien de connexion",
	LoginLinkSent:          "Consultez votre boîte de réception. Si cette adresse peut se connecter, un lien arrive.",
	LoginErrorNotAllowed:   "Cette adresse ne figure pas sur la liste d'invitations de ce Briefing.",
	LoginErrorExpired:      "Ce lien de connexion a expiré ou a déjà servi. Demandez-en un nouveau.",
	LoginErrorRateLimited:  "Trop de tentatives. Patientez quelques minutes, puis réessayez.",
	LoginErrorInvalidEmail: "Saisissez une adresse e-mail valide.",
	LoginErrorGeneric:      "La connexion a échoué. Réessayez.",

	VerifyTitle:  "Confirmer la connexion",
	VerifyBody:   "Sélectionnez le bouton pour terminer la connexion. Si vous n'avez pas demandé ce lien, fermez cette page.",
	VerifyButton: "Se connecter",
}
