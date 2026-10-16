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

	AdminTitle        string
	AdminSources      string
	AdminSource       string
	AdminKind         string
	AdminLastSuccess  string
	AdminLastError    string
	AdminNever        string
	AdminEnable       string
	AdminDisable      string
	AdminDisabled     string
	AdminJobs         string
	AdminJobKind      string
	AdminJobState     string
	AdminJobCount     string
	AdminFailures     string
	AdminNoFailures   string
	AdminSpend        string
	AdminStage        string
	AdminCalls        string
	AdminCost         string
	AdminTotal        string
	AdminCap          string
	AdminCapReached   string
	AdminNoSpendToday string

	OnboardingTitle         string
	OnboardingIntro         string
	OnboardingLanguage      string
	OnboardingInterests     string
	OnboardingInterestsHint string
	OnboardingAvoid         string
	OnboardingAvoidHint     string
	OnboardingContinue      string
	OnboardingNeedInterest  string
	OnboardingTooMany       string

	CalibrateTitle     string
	CalibrateIntro     string
	CalibrateUseful    string
	CalibrateKnew      string
	CalibrateNotForMe  string
	CalibrateRated     string
	CalibrateFinish    string
	CalibrateNoStories string

	FeedTitle        string
	FeedEmpty        string
	FeedMore         string
	FeedNewStories   string
	FeedShow         string
	FeedPublishers   string
	FeedConfidence   string
	ConfidenceHigh   string
	ConfidenceMedium string
	ConfidenceLow    string
	FeedLinkCard     string
	FeedBecause      string
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

	AdminTitle:              "Operations",
	AdminSources:            "Sources",
	AdminSource:             "Source",
	AdminKind:               "Kind",
	AdminLastSuccess:        "Last success",
	AdminLastError:          "Last error",
	AdminNever:              "never",
	AdminEnable:             "Enable",
	AdminDisable:            "Disable",
	AdminDisabled:           "off",
	AdminJobs:               "Jobs",
	AdminJobKind:            "Job",
	AdminJobState:           "State",
	AdminJobCount:           "Count",
	AdminFailures:           "Recent failures",
	AdminNoFailures:         "No recent failures.",
	AdminSpend:              "Model spend today (UTC)",
	AdminStage:              "Stage",
	AdminCalls:              "Calls",
	AdminCost:               "Cost",
	AdminTotal:              "Total",
	AdminCap:                "Daily cap",
	AdminCapReached:         "The daily cap is reached. Writing, explanations and translations wait until tomorrow; triage and embeddings continue.",
	AdminNoSpendToday:       "No model calls today.",
	OnboardingTitle:         "Set up your briefing",
	OnboardingIntro:         "Tell Briefing what you follow. It reads the sources, keeps what matters to you and leaves out the rest.",
	OnboardingLanguage:      "Language",
	OnboardingInterests:     "Topics you follow",
	OnboardingInterestsHint: "One per line, in your own words: “Rust and systems programming”, “exoplanets”, “AI safety research”.",
	OnboardingAvoid:         "Topics you do not want",
	OnboardingAvoidHint:     "Optional. One per line: “crypto prices”, “smartphone launches”.",
	OnboardingContinue:      "Continue",
	OnboardingNeedInterest:  "Add at least one topic you follow.",
	OnboardingTooMany:       "Keep it to 20 topics of at most 80 characters each.",

	CalibrateTitle:     "Calibrate",
	CalibrateIntro:     "Rate a few recent stories. Your answers tune the ranking and tell Briefing what you already know.",
	CalibrateUseful:    "Useful",
	CalibrateKnew:      "I knew this",
	CalibrateNotForMe:  "Not for me",
	CalibrateRated:     "Rated",
	CalibrateFinish:    "Open my briefing",
	CalibrateNoStories: "No stories to rate yet. You can start now; the ranking learns as you read.",

	FeedTitle:        "Today",
	FeedEmpty:        "Your briefing is being prepared. New stories appear here as they are ranked.",
	FeedMore:         "More stories",
	FeedNewStories:   "New stories are ready.",
	FeedShow:         "Show",
	FeedPublishers:   "publishers",
	FeedConfidence:   "Confidence",
	ConfidenceHigh:   "high",
	ConfidenceMedium: "medium",
	ConfidenceLow:    "low",
	FeedLinkCard:     "Not enough source text for a summary yet. Read the original:",
	FeedBecause:      "Because you follow",
}

var fr = Messages{
	AppName:          "Briefing",
	MagicLinkSubject: "Votre lien de connexion à Briefing",
	MagicLinkBody: "Ouvrez ce lien pour vous connecter à Briefing :\n\n%[1]s\n\n" +
		"Le lien ne fonctionne qu’une fois et expire dans %[2]d minutes. Si vous ne l’avez pas demandé, ignorez cet e-mail.\n",

	SkipToContent: "Aller au contenu",
	SignOut:       "Se déconnecter",

	LoginTitle:             "Connexion",
	LoginIntro:             "Briefing lit l’actualité tech et scientifique, garde l’essentiel et vous aide à le retenir.",
	LoginGitHub:            "Continuer avec GitHub",
	LoginOr:                "ou",
	LoginEmailLabel:        "Adresse e-mail",
	LoginEmailSend:         "Recevoir un lien de connexion",
	LoginLinkSent:          "Consultez votre boîte de réception. Si cette adresse peut se connecter, un lien arrive.",
	LoginErrorNotAllowed:   "Cette adresse ne figure pas sur la liste d’invitations de ce Briefing.",
	LoginErrorExpired:      "Ce lien de connexion a expiré ou a déjà servi. Demandez-en un nouveau.",
	LoginErrorRateLimited:  "Trop de tentatives. Patientez quelques minutes, puis réessayez.",
	LoginErrorInvalidEmail: "Saisissez une adresse e-mail valide.",
	LoginErrorGeneric:      "La connexion a échoué. Réessayez.",

	VerifyTitle:  "Confirmer la connexion",
	VerifyBody:   "Sélectionnez le bouton pour terminer la connexion. Si vous n’avez pas demandé ce lien, fermez cette page.",
	VerifyButton: "Se connecter",

	AdminTitle:              "Exploitation",
	AdminSources:            "Sources",
	AdminSource:             "Source",
	AdminKind:               "Type",
	AdminLastSuccess:        "Dernier succès",
	AdminLastError:          "Dernière erreur",
	AdminNever:              "jamais",
	AdminEnable:             "Activer",
	AdminDisable:            "Désactiver",
	AdminDisabled:           "désactivée",
	AdminJobs:               "Tâches",
	AdminJobKind:            "Tâche",
	AdminJobState:           "État",
	AdminJobCount:           "Nombre",
	AdminFailures:           "Échecs récents",
	AdminNoFailures:         "Aucun échec récent.",
	AdminSpend:              "Dépenses de modèles aujourd’hui (UTC)",
	AdminStage:              "Étape",
	AdminCalls:              "Appels",
	AdminCost:               "Coût",
	AdminTotal:              "Total",
	AdminCap:                "Plafond quotidien",
	AdminCapReached:         "Le plafond quotidien est atteint. La rédaction, les explications et les traductions reprennent demain ; le tri et les plongements continuent.",
	AdminNoSpendToday:       "Aucun appel de modèle aujourd’hui.",
	OnboardingTitle:         "Configurer votre briefing",
	OnboardingIntro:         "Dites à Briefing ce que vous suivez. Il lit les sources, garde ce qui compte pour vous et laisse le reste.",
	OnboardingLanguage:      "Langue",
	OnboardingInterests:     "Sujets suivis",
	OnboardingInterestsHint: "Un par ligne, avec vos mots : « Rust et programmation système », « exoplanètes », « sûreté de l’IA ».",
	OnboardingAvoid:         "Sujets à écarter",
	OnboardingAvoidHint:     "Facultatif. Un par ligne : « cours des cryptomonnaies », « sorties de smartphones ».",
	OnboardingContinue:      "Continuer",
	OnboardingNeedInterest:  "Ajoutez au moins un sujet suivi.",
	OnboardingTooMany:       "Limitez-vous à 20 sujets de 80 caractères au plus.",

	CalibrateTitle:     "Calibrer",
	CalibrateIntro:     "Notez quelques articles récents. Vos réponses ajustent le classement et indiquent à Briefing ce que vous savez déjà.",
	CalibrateUseful:    "Utile",
	CalibrateKnew:      "Je le savais",
	CalibrateNotForMe:  "Pas pour moi",
	CalibrateRated:     "Noté",
	CalibrateFinish:    "Ouvrir mon briefing",
	CalibrateNoStories: "Aucun article à noter pour l’instant. Vous pouvez commencer ; le classement apprend au fil de vos lectures.",

	FeedTitle:        "Aujourd’hui",
	FeedEmpty:        "Votre briefing se prépare. Les nouveaux articles apparaissent ici dès qu’ils sont classés.",
	FeedMore:         "Autres articles",
	FeedNewStories:   "De nouveaux articles sont prêts.",
	FeedShow:         "Afficher",
	FeedPublishers:   "médias",
	FeedConfidence:   "Fiabilité",
	ConfidenceHigh:   "élevée",
	ConfidenceMedium: "moyenne",
	ConfidenceLow:    "faible",
	FeedLinkCard:     "Pas encore assez de texte source pour un résumé. Lire l’original :",
	FeedBecause:      "Parce que vous suivez",
}
