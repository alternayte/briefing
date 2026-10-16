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

	FeedTitle      string
	FeedEmpty      string
	FeedMore       string
	FeedNewStories string
	FeedShow       string
	// FeedPublishers takes the count of publishers, 2 or more.
	FeedPublishers string
	// FeedConfidence takes the confidence word.
	FeedConfidence   string
	ConfidenceHigh   string
	ConfidenceMedium string
	ConfidenceLow    string
	FeedLinkCard     string
	// FeedBecause takes the interest text.
	FeedBecause string

	StoryBack        string
	StoryWhy         string
	StoryWhatIsNew   string
	StoryClaims      string
	StoryConcepts    string
	StorySources     string
	StoryReadText    string
	StoryNoText      string
	StoryUpdated     string
	ConceptMastered  string
	ConceptFamiliar  string
	ConceptMet       string
	ConceptNew       string
	FeedCaughtUp     string
	FeedCaughtUpHint string

	FeedbackLabel      string
	FeedbackUseful     string
	FeedbackKnew       string
	FeedbackIrrelevant string
	FeedbackMisleading string

	ExplainButton  string
	ExplainHeading string
	ExplainCap     string
	ExplainLimit   string

	SettingsTitle        string
	SettingsSave         string
	SettingsSaved        string
	SettingsBudget       string
	SettingsBudgetHint   string
	SettingsInterests    string
	SettingsAvoid        string
	SettingsAdd          string
	SettingsRemove       string
	SettingsNewInterest  string
	SettingsNewAvoid     string
	SettingsSources      string
	SettingsSubscribe    string
	SettingsUnsubscribe  string
	SettingsAddFeed      string
	SettingsFeedURL      string
	SettingsFeedAdded    string
	SettingsFeedBadURL   string
	SettingsFeedFailed   string
	SettingsNotAFeed     string
	SettingsTokens       string
	SettingsTokensHint   string
	SettingsTokenName    string
	SettingsTokenCreate  string
	SettingsTokenNew     string
	SettingsTokenRevoke  string
	SettingsTokenNone    string
	SettingsTokenLastUse string
	SettingsMCP          string
	SettingsMCPHint      string
	SettingsMCPEndpoint  string
	SettingsMCPHeader    string
	SettingsInvalid      string

	FeedTranslating   string
	ArticleTranslate  string
	ArticleTranslated string
	ArticleLimit      string
	ArticleCap        string
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

	FeedTitle:            "Today",
	FeedEmpty:            "Your briefing is being prepared. New stories appear here as they are ranked.",
	FeedMore:             "More stories",
	FeedNewStories:       "New stories are ready.",
	FeedShow:             "Show",
	FeedPublishers:       "%d publishers",
	FeedConfidence:       "%s confidence",
	ConfidenceHigh:       "high",
	ConfidenceMedium:     "medium",
	ConfidenceLow:        "low",
	FeedLinkCard:         "Not enough source text for a summary yet. Read the original:",
	FeedBecause:          "Because you follow “%s”",
	StoryBack:            "Back to today",
	StoryWhy:             "Why it matters",
	StoryWhatIsNew:       "What is new",
	StoryClaims:          "Claims and sources",
	StoryConcepts:        "Concepts",
	StorySources:         "Sources",
	StoryReadText:        "Read the extracted text",
	StoryNoText:          "No text could be extracted from this page.",
	StoryUpdated:         "Updated",
	ConceptMastered:      "mastered",
	ConceptFamiliar:      "familiar",
	ConceptMet:           "met",
	ConceptNew:           "new to you",
	FeedCaughtUp:         "You are caught up.",
	FeedCaughtUpHint:     "New stories appear here as they are ranked. More stories wait below.",
	FeedbackLabel:        "Your feedback",
	FeedbackUseful:       "Useful",
	FeedbackKnew:         "Knew it",
	FeedbackIrrelevant:   "Not relevant",
	FeedbackMisleading:   "Misleading",
	ExplainButton:        "Explain it to me",
	ExplainHeading:       "Explained for you",
	ExplainCap:           "Explanations are paused until tomorrow: today's spending cap is reached.",
	ExplainLimit:         "You have used today's explanations. More tomorrow.",
	SettingsTitle:        "Settings",
	SettingsSave:         "Save",
	SettingsSaved:        "Saved.",
	SettingsBudget:       "Stories a day",
	SettingsBudgetHint:   "How many stories your briefing shows above “More”, from 3 to 30.",
	SettingsInterests:    "Topics you follow",
	SettingsAvoid:        "Topics you do not want",
	SettingsAdd:          "Add",
	SettingsRemove:       "Remove",
	SettingsNewInterest:  "New topic to follow",
	SettingsNewAvoid:     "New topic to leave out",
	SettingsSources:      "Sources",
	SettingsSubscribe:    "Follow",
	SettingsUnsubscribe:  "Unfollow",
	SettingsAddFeed:      "Add a feed",
	SettingsFeedURL:      "RSS or Atom address",
	SettingsFeedAdded:    "The feed was added. Its stories arrive with the next poll.",
	SettingsFeedBadURL:   "Enter an http or https address.",
	SettingsFeedFailed:   "That address could not be fetched from the public internet.",
	SettingsNotAFeed:     "That page is not an RSS or Atom feed.",
	SettingsTokens:       "Access tokens",
	SettingsTokensHint:   "A token lets an AI assistant use your briefing through MCP. Keep it secret; revoke it when you stop using it.",
	SettingsTokenName:    "Name, such as “Claude on my laptop”",
	SettingsTokenCreate:  "Create a token",
	SettingsTokenNew:     "Copy this token now. It is shown only once:",
	SettingsTokenRevoke:  "Revoke",
	SettingsTokenNone:    "No tokens.",
	SettingsTokenLastUse: "last used",
	SettingsMCP:          "Connect an AI assistant",
	SettingsMCPHint:      "Add Briefing as a remote MCP server in your assistant, with a token from above as the bearer token.",
	SettingsMCPEndpoint:  "Server address",
	SettingsMCPHeader:    "Header",
	SettingsInvalid:      "That value is not valid.",
	FeedTranslating:      "Translation in progress",
	ArticleTranslate:     "Translate this article",
	ArticleTranslated:    "Machine translation",
	ArticleLimit:         "You have used today's article translations. More tomorrow.",
	ArticleCap:           "Translations are paused until tomorrow: today's spending cap is reached.",
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

	FeedTitle:            "Aujourd’hui",
	FeedEmpty:            "Votre briefing se prépare. Les nouveaux articles apparaissent ici dès qu’ils sont classés.",
	FeedMore:             "Autres articles",
	FeedNewStories:       "De nouveaux articles sont prêts.",
	FeedShow:             "Afficher",
	FeedPublishers:       "%d médias",
	FeedConfidence:       "fiabilité %s",
	ConfidenceHigh:       "élevée",
	ConfidenceMedium:     "moyenne",
	ConfidenceLow:        "faible",
	FeedLinkCard:         "Pas encore assez de texte source pour un résumé. Lire l’original :",
	FeedBecause:          "Parce que vous suivez « %s »",
	StoryBack:            "Retour à aujourd’hui",
	StoryWhy:             "Pourquoi c’est important",
	StoryWhatIsNew:       "Ce qui est nouveau",
	StoryClaims:          "Faits et sources",
	StoryConcepts:        "Concepts",
	StorySources:         "Sources",
	StoryReadText:        "Lire le texte extrait",
	StoryNoText:          "Aucun texte n’a pu être extrait de cette page.",
	StoryUpdated:         "Mis à jour",
	ConceptMastered:      "maîtrisé",
	ConceptFamiliar:      "familier",
	ConceptMet:           "rencontré",
	ConceptNew:           "nouveau pour vous",
	FeedCaughtUp:         "Vous êtes à jour.",
	FeedCaughtUpHint:     "Les nouveaux articles apparaissent ici dès qu’ils sont classés. D’autres articles attendent plus bas.",
	FeedbackLabel:        "Votre avis",
	FeedbackUseful:       "Utile",
	FeedbackKnew:         "Déjà su",
	FeedbackIrrelevant:   "Hors sujet",
	FeedbackMisleading:   "Trompeur",
	ExplainButton:        "Expliquez-moi",
	ExplainHeading:       "Expliqué pour vous",
	ExplainCap:           "Les explications reprennent demain : le plafond de dépenses du jour est atteint.",
	ExplainLimit:         "Vous avez utilisé vos explications du jour. D’autres demain.",
	SettingsTitle:        "Réglages",
	SettingsSave:         "Enregistrer",
	SettingsSaved:        "Enregistré.",
	SettingsBudget:       "Articles par jour",
	SettingsBudgetHint:   "Le nombre d’articles affichés avant « Autres articles », de 3 à 30.",
	SettingsInterests:    "Sujets suivis",
	SettingsAvoid:        "Sujets à écarter",
	SettingsAdd:          "Ajouter",
	SettingsRemove:       "Retirer",
	SettingsNewInterest:  "Nouveau sujet à suivre",
	SettingsNewAvoid:     "Nouveau sujet à écarter",
	SettingsSources:      "Sources",
	SettingsSubscribe:    "Suivre",
	SettingsUnsubscribe:  "Ne plus suivre",
	SettingsAddFeed:      "Ajouter un flux",
	SettingsFeedURL:      "Adresse RSS ou Atom",
	SettingsFeedAdded:    "Le flux a été ajouté. Ses articles arrivent avec la prochaine collecte.",
	SettingsFeedBadURL:   "Saisissez une adresse http ou https.",
	SettingsFeedFailed:   "Cette adresse est injoignable depuis l’internet public.",
	SettingsNotAFeed:     "Cette page n’est pas un flux RSS ou Atom.",
	SettingsTokens:       "Jetons d’accès",
	SettingsTokensHint:   "Un jeton permet à un assistant IA d’utiliser votre briefing via MCP. Gardez-le secret ; révoquez-le quand vous ne l’utilisez plus.",
	SettingsTokenName:    "Nom, par exemple « Claude sur mon portable »",
	SettingsTokenCreate:  "Créer un jeton",
	SettingsTokenNew:     "Copiez ce jeton maintenant. Il n’est affiché qu’une fois :",
	SettingsTokenRevoke:  "Révoquer",
	SettingsTokenNone:    "Aucun jeton.",
	SettingsTokenLastUse: "dernière utilisation",
	SettingsMCP:          "Connecter un assistant IA",
	SettingsMCPHint:      "Ajoutez Briefing comme serveur MCP distant dans votre assistant, avec un jeton ci-dessus comme jeton porteur.",
	SettingsMCPEndpoint:  "Adresse du serveur",
	SettingsMCPHeader:    "En-tête",
	SettingsInvalid:      "Cette valeur n’est pas valide.",
	FeedTranslating:      "Traduction en cours",
	ArticleTranslate:     "Traduire cet article",
	ArticleTranslated:    "Traduction automatique",
	ArticleLimit:         "Vous avez utilisé vos traductions d’articles du jour. D’autres demain.",
	ArticleCap:           "Les traductions reprennent demain : le plafond de dépenses du jour est atteint.",
}
