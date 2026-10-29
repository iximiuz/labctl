# General Style Guides

Never use em dashes (`—`). Use hyphens (`-`) instead.

Write in simple, clear, and concise "International English". Avoid fancy words and overly complex sentences.
Words are expensive! Writing is easy now - but reading is still not. Keep things short yet clear.
Do not produce sentences and paragraphs only for the sake of creating an impression of a more solid write-up.
Write only as much as needed to explain the idea or make the point and no more.
Don't reiterate or strengthen the point routinely. Only the key points are worth repeating,
but it should happen naturally and ideally from different places of the write-up.

Do not use silly hooks like:

- "Not every TCP server waits for you to speak first."

Avoid questionable claims like:

- "Most of the time we picture a TCP server as something that politely waits for a client to send a request before it does anything."

Avoid silly phrases like:

- "Two pitfalls bite people here."
- "Socket activation flips the usual lifecycle on its head."
- "Wire up (a systemd service, a process, etc.)"

Avoid the "X, not Y" contrast phrasing (and its "not X, but Y" / "X - not Y" variants). It reads as an AI cliche. State what the thing is, plainly, instead of what it is not:

- Bad: "An error message is a conversation, not a catastrophe."
- Good: "Nothing broke: an error message is just the command telling you what it could not do."
- Bad: "Think of the prompt as an invitation, not a warning."
- Good: "The prompt means the shell is ready for the next command."

The same goes for the "It's not about X, it's about Y" pattern.

Avoid sentence structures that require a semicolon. A semicolon joins two independent clauses that could each stand alone, so split them into two sentences instead. Semicolons in prose are another AI cliche and make the text harder to read in a narrow pane:

- Bad: "Short options are quicker to type; long options are easier to read later."
- Good: "Short options are quicker to type. Long options are easier to read later."

Semicolons inside code, shell commands, and YAML are fine.

Avoid shortcut sentences: clipped, telegram-style fragments that drop the subject or the verb to save a few words, and "label: detail" constructions used in place of a sentence. Every sentence should have a subject and a verb and read as a complete thought, even in short task feedback:

- Bad: "Identical output."
- Good: "The output is identical."
- Bad: "One down."
- Good: "That is one of the three done."
- Bad: "Three letters: tty."
- Good: "The command is `tty`."
- Bad: "Two commands, one line."
- Good: "Both commands ran from a single line."

The exception is a short label that is meant to be a label, such as a table cell, a diagram caption, or a list item that is a bare name.

Avoid the word "reflex" (and "reflexes") when writing about command-line skills or muscle memory. Describe the skill or the habit plainly instead:

- Bad: "This module drills `type` until asking becomes a reflex."
- Good: "This module drills `type` until asking is the first thing you do."
- Bad: "These reflexes matter because every command is documented the same way."
- Good: "These skills matter because every command is documented the same way."

Name the thing first, then explain it. Do not describe a value (a path, a command, a character, a name) before revealing it, and do not hang the explanation on the instruction as an appositive. The reader should get the concrete token in a plain sentence and the meaning in the next one. A build-up before the reveal ("the shortest one there is:", "three letters long:", "a slash, then tmp") reads as a riddle, and a mid-sentence aside delays the instruction the reader is trying to follow:

- Bad: "Move to the root directory, the top of the tree. Its path is the shortest one there is: a single `/`."
- Good: "Move to the root directory. Its path is `/`, which is the top of the filesystem tree."
- Bad: "Every Linux machine has a directory for files that nobody needs to keep: `/tmp`."
- Good: "Every Linux machine has a `/tmp` directory for files that nobody needs to keep."
- Bad: "First move to `/var`, the directory for data that changes while the machine runs, such as logs."
- Good: "First move to `/var`. It holds data that changes while the machine runs, such as logs."
- Bad: "Its full path is a slash, then tmp."
- Good: "Its full path is `/tmp`."

A short clarification in parentheses right after the token is fine: "`..` (two dots)", "`cd -` (a single dash as the argument)".

Avoid the "<Something> is exactly what ..." pattern and its relatives ("exactly like", "exactly the same as", "is precisely what"). It is another AI writing giveaway: the word adds emphasis without adding information. Say what the thing is, or name the two things and state that they are the same:

- Bad: "The `auth` value in the file is exactly what the client puts after `Basic` in the `Authorization` header."
- Good: "The `auth` value in the file is the string that follows `Basic` in the `Authorization` header."
- Bad: "A tag list is exactly what a container registry returns."
- Good: "A container registry returns the same kind of tag list."

The word "exactly" is fine when precision is the point, such as "byte for byte" instructions: "Save the response body exactly as the server sent it."

Keep image alt texts (the `:alt:` of an `::image-box`) short. A few words that name what the image shows are enough. Do not retell the diagram or explain the mechanism in the alt text. The surrounding prose does that:

- Bad: "The --add-host flag adds a billing.acme.corp entry to the container's own /etc/hosts file, so the app sends its requests to the request sink."
- Good: "Redirecting the container's requests with --add-host."
