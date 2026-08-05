export type PolicyInfoKind = 'privacy' | 'terms'
export type InfoKind = 'product' | PolicyInfoKind | 'acknowledgements'

interface InfoSection {
  title: string
  body?: string
  items?: string[]
}

export interface InfoContent {
  title: string
  intro: string
  sections: InfoSection[]
}

type Translate = (key: string) => string

export function getAboutInfoContent(infoKind: InfoKind, translate: Translate): InfoContent {
  if (infoKind === 'product') return {
    title: translate('settingsAbout.product.title'),
    intro: translate('settingsAbout.product.intro'),
    sections: [
      { title: translate('settingsAbout.product.positionTitle'), body: translate('settingsAbout.product.positionBody') },
      { title: translate('settingsAbout.product.featuresTitle'), items: [translate('settingsAbout.product.featureAccounts'), translate('settingsAbout.product.featureMail'), translate('settingsAbout.product.featureSearch'), translate('settingsAbout.product.featureContacts'), translate('settingsAbout.product.featurePrivacy'), translate('settingsAbout.product.featureBackup')] },
      { title: translate('settingsAbout.product.dataTitle'), body: translate('settingsAbout.product.dataBody') },
    ],
  }
  if (infoKind === 'privacy') return {
    title: translate('settingsAbout.privacy.title'),
    intro: translate('settingsAbout.privacy.intro'),
    sections: [
      { title: translate('settingsAbout.privacy.noCollectionTitle'), items: [translate('settingsAbout.privacy.noCollectionPersonal'), translate('settingsAbout.privacy.noCollectionMail'), translate('settingsAbout.privacy.noCollectionTracking'), translate('settingsAbout.privacy.noCollectionAds'), translate('settingsAbout.privacy.noCollectionSale')] },
      { title: translate('settingsAbout.privacy.localTitle'), items: [translate('settingsAbout.privacy.localMail'), translate('settingsAbout.privacy.localAccount'), translate('settingsAbout.privacy.localContacts'), translate('settingsAbout.privacy.localSettings'), translate('settingsAbout.privacy.localLogs'), translate('settingsAbout.privacy.localBackups')] },
      { title: translate('settingsAbout.privacy.securityTitle'), body: translate('settingsAbout.privacy.securityBody') },
      { title: translate('settingsAbout.privacy.retentionTitle'), body: translate('settingsAbout.privacy.retentionBody') },
      { title: translate('settingsAbout.privacy.contactTitle'), body: translate('settingsAbout.privacy.contactBody') },
    ],
  }
  if (infoKind === 'terms') return {
    title: translate('settingsAbout.terms.title'),
    intro: translate('settingsAbout.terms.intro'),
    sections: [
      { title: translate('settingsAbout.terms.descriptionTitle'), body: translate('settingsAbout.terms.descriptionBody') },
      { title: translate('settingsAbout.terms.responsibilitiesTitle'), items: [translate('settingsAbout.terms.responsibilityCredentials'), translate('settingsAbout.terms.responsibilityDevice'), translate('settingsAbout.terms.responsibilityLaw'), translate('settingsAbout.terms.responsibilityProvider'), translate('settingsAbout.terms.responsibilityBackup')] },
      { title: translate('settingsAbout.terms.useTitle'), body: translate('settingsAbout.terms.useBody') },
      { title: translate('settingsAbout.terms.disclaimerTitle'), body: translate('settingsAbout.terms.disclaimerBody') },
      { title: translate('settingsAbout.terms.thirdPartyTitle'), body: translate('settingsAbout.terms.thirdPartyBody') },
      { title: translate('settingsAbout.terms.contactTitle'), body: translate('settingsAbout.terms.contactBody') },
    ],
  }
  return {
    title: translate('settingsAbout.acknowledgements.title'),
    intro: translate('settingsAbout.acknowledgements.intro'),
    sections: [
      { title: translate('settingsAbout.acknowledgements.technologyTitle'), items: [translate('settingsAbout.acknowledgements.technologyDesktop'), translate('settingsAbout.acknowledgements.technologyEditor'), translate('settingsAbout.acknowledgements.technologyInterface'), translate('settingsAbout.acknowledgements.technologyData')] },
      { title: translate('settingsAbout.acknowledgements.communityTitle'), body: translate('settingsAbout.acknowledgements.communityBody') },
      { title: translate('settingsAbout.acknowledgements.licenseTitle'), body: translate('settingsAbout.acknowledgements.licenseBody') },
    ],
  }
}
