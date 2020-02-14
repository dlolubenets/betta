create table users
(
    id      serial primary key,
    balance int  not null default 0,
    email   text NOT NULL unique
);

create table source_types
(
    id    serial primary key,
    value text not null
);


create table transactions
(
    external_id text primary key,
    user_id     int       not null references users (id),
    type        text      not null,
    amount      int       not null,
    source_type int       not null references source_types (id),
    processed   bool      not null default false,
    created_at  timestamp not null default now()

);

insert into users (email)
values ('default_player@gmail.com');

insert into source_types (value)
values ('game'),
       ('server'),
       ('payment');