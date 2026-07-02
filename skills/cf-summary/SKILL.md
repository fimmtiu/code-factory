---
name: cf-summary
description: "Use when the user needs to get oriented in a new codebase for the first time. Triggers on: summarize this project, describe this project, document this project, /cf-summary."
user-invocable: true
---

# Codebase summarizer

This is a new codebase that neither you nor the user have seen before, and we want to get the user up to speed on all
the various parts of the codebase and how they work. Your audience is a technically skilled staff developer. Generate a
summary of what this project does, how it works, and where to find all of the important parts. Your summary should be
based on the actual code, not the documentation — the documentation is useful for orienting yourself, but you must be
able to back up anything you output with a reference to a specific location in the code.

**Do not document anything that would be obvious to a staff developer.** Don't explain things like basic terminology,
commonly used acronyms, ubiquitous tools, or well-known design patterns. The reader already knows these things, so the
explanation would just be noise.

**Write all output in English.** Do not emit non-Latin-script characters unless quoting source material.

## Output format

All of your output should go into an HTML file called "SUMMARY.html" in the root of the project. If this file already
exists, truncate it first. Important terminology, names of classes or methods, and XXX


should be hyperlinked with relative paths to the file in the codebase where they're defined. Link each term, class,
method, or XXX only once, the first time it's mentioned — after that, just write it in plain text.

You are free to supplement the text with ASCII-art diagrams where a visual representation would be helpful, but it's not
required.

### Structure of output

The document should be structured with the following sections:

#### High-level overview

What does this codebase do? How mature is it — does it actually do what it's supposed to, or is it still under
development? If it's not complete, what parts are yet to be implemented?

#### Important concepts and terms

What domain-specific concepts and terms is this codebase built around? Create a glossary of ubiquitous language for
these things. If some terms are used inconsistently or ambiguously, mention that.

What conventions does this codebase adhere to for common things? Describe any noteworthy ones that would help an
experienced user contribute code that doesn't break existing conventions.

#### Architecture overview

What language is it in? What important tools, libraries, or cloud services does it rely upon? Give a brief overview of
the on-disk structure of the codebase that would help someone find where to look for a particular piece of code or
functionality. Mention any design patterns that are structurally important or frequently used. If the codebase contains
any isolated bounded contexts, mention that and describe the boundaries.

#### Data model

Where is the data stored? What's the format of the stored data? Give a list of the most important types of data in the
system and how they relate to each other. Examples of what the data looks like are helpful here.

#### Important classes

What are the most important classes in the system? Where do they live in the codebase? How do they relate to each other?
How are they implemented?

#### Opportunities for improvement

Are there parts of this codebase that seem inefficient or confused? What would be some easy wins in terms of code
cleanup, adding missing functionality, improving tests, or making it easier to understand?

#### How to do basic things

Explain how to do all of the following (if relevant to the codebase):

* How to examine the data directly
* How to create test data
* How to clear the database in development mode
* How to run automated tests
* How to test the system manually
* How to debug problems locally
