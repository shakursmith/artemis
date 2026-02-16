This project directory contains two repos for the cool IOS app we're building. The frontend repo is `/apollo` and the backend repo is `/artemis`. The frontend should be xcode x swift x swift UI. The backend should be golang. The other stuff we will decide together as we go along.

Recommend test files so we can have decent coverage for both repos.

Lots of comments explaining what's going on.

# Big Idea

We're building a cool app the re-envisions the way we use our phones. Today you open a smartphone and there's a grid full of apps. Individually they all serve different purposes but typically the experience of using a smartphone feels very disjointed. You have to open different apps to do different things and they don't really talk to each other.

I want to build something that's more environment based. Depending on what environment you are in (living room, work, car, gym), from within the app you are able to interact with different apps and services that are relevant to that environment. So when you're in the living room, you might want to interact with your smart TV, your music streaming service, and your smart lights. When you're at work, you might want to interact with your calendar, email, and task management apps. When you are at a restaurant, you might want to interact with your calendar, maps, and food delivery apps. The idea is that the app serves as a sort of "home base" for all of your interactions with your phone, and it surfaces the most relevant apps and services based on your current environment.

To build this we will need to prioritize integrating with a lot of different apps and services. We will need to build a backend that can handle all of these integrations and a frontend that can surface the relevant apps and services based on the user's current environment. And will want to prioritize apps that already have public facing APIs to make the integrations easier.

All of the features we will be developing end to end. Each repo is it's own git instance, so let's work in sizable chunks so we can commit often.
