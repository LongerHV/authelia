---
title: "Internationalization"
description: "Authelia Internationalization Implementation"
lead: "Implementation of internationalization will make Authelia more accessible to more people."
date: 2022-03-20T12:52:27+11:00
draft: false
images: []
menu:
  roadmap:
    parent: "active"
weight: 230
toc: true
---

This can easily be done in the web interface and automatically adapt to the users browser.

## Stages

This section represents the stages involved in implementation of this feature. The stages are either in order of
implementation due to there being an underlying requirement to implement them in this order, or in their likely order
due to how important or difficult to implement they are.

### Initial Implementation

{{< roadmap-status stage="complete" version="v4.34.0" >}}

This stage will add the ability to easily translate the web interface in all views.

### Crowd Translation Service

{{< roadmap-status stage="complete" >}}

This stage will configure the Authelia repository to be easily translatable via a crowd sourced translation platform.

*__Implemented:__ You can now help translate __Authelia__ by checking out the
[Translations Contributing Guide](../../contributing/prologue/translations.md).*

### Picker

{{< roadmap-status >}}

Add a language picker to the web interface. The picker will be a per-browser choice which overrides the browser
language advertisement as the language of choice for that browser. The information will be stored in the browser
[local storage](https://developer.mozilla.org/en-US/docs/Web/API/Window/localStorage) implementation.

### Ongoing

There will be an ongoing effort to keep the interface translated.
